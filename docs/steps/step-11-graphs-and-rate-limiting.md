# Step 11 — Graphs & rate limiting: topological sort and the token bucket

| | |
|---|---|
| **Build** | `graph.TopoSort` (`internal/graph/toposort.go`), `ratelimit.TokenBucket` (`internal/ratelimit/token_bucket.go`) |
| **Given** | `graph.CycleError`, `graph.LoadPipeline`, `ratelimit.ErrInvalidTokens`, `clock.Clock`, `clock.Sleeper` |
| **Tests** | `internal/graph/graph_test.go` (10) + `internal/ratelimit/ratelimit_test.go` (11) — 21 tests |
| **Run** | `go test ./internal/graph ./internal/ratelimit -run Step11 -race -v` |
| **You'll learn** | Kahn's algorithm, in-degree counting, cycle detection, deterministic output, lazy refill, floating-point care, rounding up durations, an injectable clock |
| **Time** | ~75–100 min |

## 1. The story

Our nightly data pipeline (the `pipeline_tasks` / `pipeline_dependencies` tables) has ten tasks: *clean_orders* needs *extract_orders*, *publish_dashboard* needs two others… We need a valid **run order**, and a clear error if someone creates a loop. Separately, the warehouse API allows bursts but only so many requests per second on average — we need a **rate limiter**.

## 2. Concept corner

### Topological sort — Kahn's algorithm

A *dependency graph* has an edge from each task to what it needs. A **topological order** lists every task **after** all its dependencies. One exists **iff there is no cycle** (a needs b needs a ⇒ impossible).

Kahn's algorithm in four moves:

1. For every node count its unmet dependencies (**in-degree**) and remember who is waiting on it (**dependents**).
2. Queue every node with in-degree 0 — nothing blocks them.
3. Take a ready node, append it to the output, decrease the in-degree of everything waiting on it; whoever reaches 0 becomes ready.
4. If the output is shorter than the graph, the leftovers are stuck behind a cycle.

O(V + E). **Cycle detection comes for free** — no separate search.

**Determinism.** Many valid orders usually exist. Logs and tests want *one* predictable answer, so whenever several nodes are ready take the alphabetically smallest: the ready-queue is a min-heap of strings. (In Go, map iteration order is **random on purpose** — an algorithm that depends on it gives different answers on different runs. That's why determinism has to be designed in.) Trick: embed `sort.StringSlice` in a struct and add only `Push`/`Pop` to get a `heap.Interface` for free.

**Subtleties the tests check:** nodes that only appear as someone's dependency are part of the graph; duplicate dependency entries must not be double-counted (that would make the in-degree too high and fake a cycle); a self-dependency is a cycle; `Unresolved` includes everything *downstream* of a cycle, not just the cycle itself.

**Why `TopoSort` returns a `*CycleError`.** A typed error carrying data (the unresolved nodes). Callers use `errors.As`. Same pattern as Step 06's `OutOfStockError`.

### Token bucket

A bucket holds up to *capacity* tokens, starts **full**, and refills continuously at *r* tokens/second. Each operation spends tokens; none left, no operation. That allows **bursts** (up to capacity) while enforcing an **average rate** *r*.

**The trick: no timer.** Refill *lazily*: whenever anyone touches the bucket, compute `elapsed × rate`, add it (capped at capacity), remember "now". No goroutine, no ticker, no drift — and trivially testable with `FakeClock`.

**Fractions matter.** At 2 tokens/s, after 250 ms the bucket holds **half** a token — not enough for one. Tokens are a `float64`.

**Waiting politely.** `Wait` loops: try → if it fails, ask *how long until enough*, `sleeper.Sleep(ctx, thatLong)`, try again. **Round that duration up** (`math.Ceil` on the nanoseconds): rounding down can wake you just *before* the token exists and spin. (Tests use `RecordingSleeper` with the fake clock, which advances by each requested sleep, simulating real time.)

**Impossible requests.** Asking for more than `capacity` tokens could never succeed ⇒ `ErrInvalidTokens` instead of waiting forever.

## 3. Requirements

### `TopoSort(dependsOn)`
`dependsOn` maps a node to the nodes it **depends on**.
- Each node exactly once, after all its dependencies; dependency-only nodes included; duplicate entries ignored.
- Ties ⇒ alphabetical.
- Cycle (self-dependency included) ⇒ `*CycleError`, `Unresolved` = every node that could not be ordered, sorted.
- Empty graph ⇒ empty non-nil slice.
- On the seeded pipeline: `extract_customers, extract_orders, clean_orders, extract_products, compute_best_sellers, join_orders_customers, compute_revenue, publish_dashboard, send_report_email, archive_raw` — work out *why* `extract_products` isn't third. (Hint: when `extract_orders` is placed, `clean_orders` becomes ready.)

### `TokenBucket`
- `NewTokenBucket(capacity, refillPerSecond, clock)`: `capacity < 1` or rate `<= 0` ⇒ error. Starts full.
- `Available()` (after refill, fractional), `TryTake(n)` (all-or-nothing: a failed attempt takes **nothing**), `TimeUntil(n)` (0 if available; else rounded **up**), `Wait(ctx, sleeper, n)`.
- `n < 1` or `n > capacity` ⇒ `ErrInvalidTokens` (wrap it with context using `%w`).
- Thread-safe: 100 tokens, frozen clock, 8 goroutines × 50 attempts ⇒ exactly 100 succeed.

## 4. Hints

<details><summary>Hint 1 — Kahn's data structures</summary>

`unmet := map[string]int{}`, `dependents := map[string][]string{}`, and a local `register(node)` that adds a node to `unmet` if absent. Calling it for both the key *and* each dependency handles dependency-only nodes.
</details>

<details><summary>Hint 2 — duplicates and the stuck list</summary>

Keep a per-node `seen := map[string]bool{}` while iterating its dependencies. For the cycle report, collect `unmet` keys not in the `placed` set, then `slices.Sort`.
</details>

<details><summary>Hint 3 — the string heap</summary>

```go
type stringHeap struct{ sort.StringSlice }
func (h *stringHeap) Push(x any) { h.StringSlice = append(h.StringSlice, x.(string)) }
func (h *stringHeap) Pop() any   { old := h.StringSlice; n := len(old); x := old[n-1]; h.StringSlice = old[:n-1]; return x }
```
`StringSlice` already provides `Len`, `Less`, `Swap`.
</details>

<details><summary>Hint 4 — lazy refill</summary>

```go
now := b.clock.Now()
if elapsed := now.Sub(b.lastRefill).Seconds(); elapsed > 0 { b.tokens = math.Min(float64(b.capacity), b.tokens+elapsed*b.refillPerSecond) }
b.lastRefill = now
```
Call it at the start of every public method, **inside** the lock.
</details>

<details><summary>Hint 5 — rounding the wait up</summary>

`time.Duration(math.Ceil(seconds * float64(time.Second)))`.
</details>

## 5. Stretch challenges

1. **Levels:** `Levels(...)` returning groups that can run **in parallel** (everything whose dependencies are all in earlier groups). Execute each level with Step 09's `concurrency.Map`, level after level — a mini workflow engine. 🎉
2. Report only the nodes **on** a cycle (not those downstream) — look up Tarjan's strongly-connected-components algorithm.
3. A **sliding-window** limiter ("at most 100 calls in any 60 s"). Compare memory and window-edge behaviour with the token bucket.
4. A `RateLimited(handler jobs.Handler, bucket *TokenBucket) jobs.Handler` wrapper (Step 07 + the decorator idea) that calls `Wait` before delegating.
5. Replace the mutex with `sync/atomic`. Where does it get hard? (Two fields — `tokens` and `lastRefill` — must change together.)
6. Compare with `golang.org/x/time/rate`: which of its concepts did you just build?

## 6. Reflect

- What would change in the output if the ready-queue were a plain FIFO slice instead of a heap — and why would that matter for tests and debugging?
- Token bucket vs "leaky bucket" vs fixed-window counter: which allows a burst of 2× the limit at a window boundary?
- You injected `Clock` and `Sleeper`. List the tests that would have taken real seconds — or been flaky — without them.
- Why is `Available()` a method that takes the lock and refills, rather than just returning a field?

## 7. Done when

`go test ./internal/graph ./internal/ratelimit -run Step11 -race` → **ok**. Then run the **whole** suite: `go test -race ./...` → all 189 green. 🏁
