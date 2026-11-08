# Step 10 — Heaps and caches: Top-K and LRU

| | |
|---|---|
| **Build** | `TopK` (`internal/algo/topk.go`), `LRU` (`internal/algo/lru.go`), `analytics.BestSellers` (`internal/analytics/analytics.go`) |
| **Given** | `algo.Scored` |
| **Tests** | `internal/algo/algo_test.go` (15) + `internal/analytics/analytics_test.go` (5) — 20 tests |
| **Run** | `go test ./internal/algo ./internal/analytics -run Step10 -race -v` |
| **You'll learn** | `container/heap` (implementing its interface), `container/list`, generic data structures with type parameters and `cmp.Ordered`, Big-O thinking, deterministic tie-breaking, streaming aggregation |
| **Needs** | Step 09's `SQLiteReader` for one test (`BestSellers_WorksOnTheRealSeededDatabase`) |
| **Time** | ~90–120 min |

## 1. The story

Marketing wants a "Top 10 best-sellers" widget computed from millions of order lines. And catalogue lookups need a *bounded* cache that doesn't grow forever. Two classic data-structure problems, each with a naive solution that works on ten items and collapses on ten million.

## 2. Concept corner

### Top-K with a heap

Naive: sort everything, take K → **O(n log n)** time and all *n* items in memory. Better: keep only the **best K seen so far** in a **min-heap** whose root is the *worst* of those K:

```
Add:  heap not full           → push
      better than the root    → replace the root, heap.Fix(h, 0)
      otherwise               → ignore                     (O(log K) per kept item, O(1) otherwise)
```
**O(n log K)** time, **O(K)** memory, one pass — and because items arrive through `Add`, millions of rows can stream past without being held.

**`container/heap` doesn't give you a heap type; it gives you algorithms for *your* type.** You implement five methods on a slice type and call `heap.Push/Pop/Fix`:

```go
type worstFirst[T cmp.Ordered] []Scored[T]
func (h worstFirst[T]) Len() int           { return len(h) }
func (h worstFirst[T]) Less(i, j int) bool { return worse(h[i], h[j]) }   // min-heap: worst at the root
func (h worstFirst[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *worstFirst[T]) Push(x any)        { *h = append(*h, x.(Scored[T])) }   // pointer receiver: it changes the slice header
func (h *worstFirst[T]) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }
```
Note `Push`/`Pop` are called *by the heap package*; you call `heap.Push(&h, x)`, not `h.Push(x)`.

**Generics.** `TopK[T cmp.Ordered]` works for ints, strings, anything comparable with `<`. A type parameter list appears after the type name; Go infers it at call sites where it can (`algo.NewTopK[int64](k)` when it can't).

**Ties must be deterministic.** "Best" = higher score first; equal scores ⇒ the *smaller* item first. Define **one** function — "is A worse than B?" (lower score, or equal score and *larger* item) — and use it for the heap's `Less`, for the replace-the-root decision, *and* for the final sort. Two slightly different comparisons is *the* classic bug; a randomised test compares you with a full sort on 5 000 items with many ties.

### LRU cache

Lookup, insert **and** eviction all O(1). One structure can't do both jobs, so combine two:

```
map:   "a" ─▶ ┐   "b" ─▶ ┐   "c" ─▶ ┐
list:      [ c ] ⇄ [ b ] ⇄ [ a ]         front = most recent, back = least recent (evict me first)
```
`map[K]*list.Element` gives O(1) lookup; the element lets `list.MoveToFront` / `Remove` run in O(1) with **no searching** — *that* is why the map stores elements, not values. `Get` and `Put` of an existing key both refresh recency; `Contains` is a **peek**.

**One mutex.** Even `Get` *modifies* the list (move-to-front), so a read-write lock can't help: `sync.Mutex` + `defer mu.Unlock()`. The concurrency test hammers it from 8 goroutines under `-race`.

**Zero values.** `Get` on a miss returns `var zero V, false` — the idiomatic "found" boolean instead of a nil pointer.

## 3. Requirements

### `TopK[T cmp.Ordered]`
- `NewTopK(k)`: `k < 0` ⇒ error; `k == 0` allowed (keeps nothing).
- `Add(item, score)`; `Results()`: at most `k` entries, **best first** (higher score; ties ⇒ smaller item), an empty non-nil slice if none. `Results` is repeatable and `Add` may continue afterwards (copy before sorting; don't destroy the heap).

### `LRU[K comparable, V any]`
`NewLRU(capacity)` (`< 1` ⇒ error); `Get` (hit ⇒ most recent); `Put` (new key evicts the least recent when full; existing key updates value, refreshes, doesn't grow); `Contains` (no refresh); `Len`; safe for concurrent use.

### `analytics.BestSellers(ctx, reader, k)`
Stream with `reader.ForEach`, sum units per product in a map, feed `TopK`, return `[]ProductUnits` best first (ties ⇒ smaller product id). `k < 0` ⇒ error; reader errors (incl. cancellation) returned as they are. Seed result: **(18, 47), (8, 28), (14, 18)**; products 14 and 16 tie at 18 and 14 wins on id. Because Go randomises map iteration, a test runs it 20 times to prove the result is deterministic anyway.

## 4. Hints

<details><summary>Hint 1 — the one comparison</summary>

```go
func worse[T cmp.Ordered](a, b Scored[T]) bool {
    if a.Score != b.Score { return a.Score < b.Score }
    return a.Item > b.Item            // the LARGER item is worse
}
```
Use `worse(h[i], h[j])` in `Less`; `worse(t.best[0], entry)` to decide replacement; `worse(out[j], out[i])` in `sort.Slice` for best-first.
</details>

<details><summary>Hint 2 — replacing the root</summary>

`t.best[0] = entry; heap.Fix(&t.best, 0)` — cheaper than Pop + Push.
</details>

<details><summary>Hint 3 — LRU entries</summary>

Store `*entry{key, value}` in the list's `Value` (type `any`); you need the key when evicting from the back so you can `delete(c.items, key)`. Read it back with `el.Value.(*entry[K, V])`.
</details>

<details><summary>Hint 4 — Put</summary>

Existing: update value + `MoveToFront`. New: if `Len() == capacity`, `Back()`, `Remove`, `delete` from the map; then `PushFront` and store the element in the map.
</details>

## 5. Stretch challenges

1. `LRU.GetOrLoad(key, loader)` with **stampede protection** — you built exactly that idea in Step 04.
2. A per-entry time-to-live using `clock.Clock`. Eager or lazy expiry — and why lazy?
3. Top-K **per category** (group products by category, top 3 each). Total complexity?
4. Replace `cmp.Ordered` with an explicit `less func(a, b T) bool` parameter. More flexible; what do you lose?
5. Benchmark `TopK` against sorting everything (`func BenchmarkTopK(b *testing.B)`, `go test -bench .`) with 1 million items and K = 10.

## 6. Reflect

- For which K does the heap stop being worth it? (What if K == n?)
- Why does the map store `*list.Element` and not the value? What would `Get`'s complexity be otherwise?
- `Push` and `Pop` have pointer receivers but `Len`, `Less`, `Swap` don't. Why?
- Is `LRU.Len()` safe to call without the lock? Run it with `-race` after removing the lock and see.

## 7. Done when

`go test ./internal/algo ./internal/analytics -run Step10 -race` → **ok**.
