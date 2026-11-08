# Step 09 — Bounded concurrency & streaming rows

| | |
|---|---|
| **Build** | `concurrency.Map` (`internal/concurrency/map.go`), `sales.SQLiteReader.ForEach` (`internal/sales/sqlite_reader.go`) |
| **Tests** | `internal/concurrency/concurrency_test.go` (11) + `internal/sales/sales_test.go` (7) — 18 tests |
| **Run** | `go test ./internal/concurrency ./internal/sales -run Step09 -race -v` |
| **You'll learn** | Generics, worker pools, `sync.Once` for "first error wins", cancel-then-wait (no goroutine leaks), why you must never return while goroutines still run, callback iteration vs channels vs `iter.Seq`, connection leaks |
| **Time** | ~90–120 min — the trickiest concurrency step |

## 1. The story

Two problems that look unrelated:

1. We must call a rate-limited warehouse API for 500 products. One by one takes forever; all 500 at once gets us banned. We want "at most 5 at a time".
2. We want to analyse every order line ever sold. Loading a million rows into a slice first would exhaust memory; we want to **stream** them.

## 2. Concept corner

### Part 1 — `concurrency.Map`

You are building a small, generic cousin of `golang.org/x/sync/errgroup`.

**A worker pool.** `maxParallel` goroutines read item indexes from a channel; a *feeder* goroutine hands the indexes out:

```go
indexes := make(chan int)
go func() {                         // feeder — the ONLY sender, so it is the one that may close
    defer close(indexes)
    for i := range items {
        select {
        case indexes <- i:
        case <-runCtx.Done(): return
        }
    }
}()
for range min(maxParallel, len(items)) {
    go func() { for i := range indexes { ... results[i] = r ... } }()
}
```
`results[i]` is written by exactly one goroutine per index, so no lock is needed; Go 1.22's per-iteration loop variables make `go func() { … i … }()` safe.

**Fail fast, and the *first* failure wins.** On the first error: cancel the context the others use and start nothing new. But several calls can fail — the first to fail is the *root cause*; the others fail later *because you cancelled them* (`context.Canceled`). Which one do you report? Not "whichever I look at first" (e.g. the lowest index). Record the first as it **happens**:

```go
fail := func(err error) { once.Do(func() { firstErr = err; cancel() }) }
```
Recording *and* cancelling inside the same `Once` means no follow-up error can ever overtake the real one — they only exist after the cancel.

**Never return while goroutines you started are still running.** If `Map` returns on the first error while other calls are mid-flight, those goroutines (a) leak and (b) keep writing into `results` — memory you've already handed back: a data race. Always `wg.Wait()` before returning. A test makes the other calls slow to clean up after cancellation and checks none is still running when `Map` returns.

**Parent vs derived context.** `runCtx, cancel := context.WithCancel(ctx)`: cancelling `runCtx` (on failure) must not touch the caller's `ctx`; but if the *caller* cancels, `runCtx` is cancelled too. After the wait, if there was no failure, check `ctx.Err()`.

### Part 2 — streaming

`ForEach(ctx, yield)` is **callback iteration**. Why not return a `[]SoldItem`? Memory. A channel? Needs a goroutine, and if the consumer stops reading the goroutine leaks. A callback is simple and leak-proof: the *reader* owns the loop and can clean up however it ends. (Go 1.23's range-over-func iterators, `iter.Seq`, are this same idea with nicer syntax: Stretch 3.) `ErrStop` follows the `filepath.SkipDir` convention: "I've seen enough — stop, and don't call it an error".

**The connection leak.** `defer rows.Close()` immediately after `QueryContext`. Return early, get cancelled, hit a callback error: the deferred close still releases the connection. Forget it and the pool slowly empties until everything hangs. One test runs with a pool of **one** connection: stop early twice, then query again — a leak freezes it.

**`rows.Err()`.** `rows.Next()` returning `false` means "done" *or* "error". Always check `rows.Err()` after the loop.

## 3. Requirements

### `Map(ctx, items, maxParallel, fn)`
1. `results[i]` belongs to `items[i]`, whatever order calls finish in.
2. At most `maxParallel` calls at once — but that much parallelism is really used.
3. `maxParallel < 1` ⇒ `ErrInvalidParallelism`.
4. **Fail fast:** cancel the others' context, start no new calls, return the **first failure that happened** (results `nil`).
5. Caller cancels ⇒ return its error; already-cancelled ⇒ `fn` never called.
6. **Never return while a call is still running.**
7. Empty input ⇒ empty non-nil slice, nil error.

### `SQLiteReader.ForEach(ctx, yield)`
Stream `(product_id, quantity)` for every line of every **non-cancelled** order, ordered by line id (the SQL is in the doc comment), row by row. `yield` returns `ErrStop` ⇒ stop, return `nil`; other error ⇒ stop, return it. Check `ctx.Err()` inside the loop (cancelled mid-way ⇒ return it right away). `defer rows.Close()`; return `rows.Err()`.

## 4. Hints

<details><summary>Hint 1 — Map's skeleton (try without it first)</summary>

```text
validate; return early on cancelled ctx / empty input
runCtx, cancel := WithCancel(ctx); defer cancel()
results := make([]R, len(items)); once, firstErr
indexes channel + feeder goroutine (defer close(indexes))
N workers: for i := range indexes { if runCtx.Err() != nil { return }; r, err := fn(runCtx, items[i]); if err != nil { fail(err); return }; results[i] = r }
wg.Wait()
if firstErr != nil → return nil, firstErr ; if ctx.Err() != nil → return nil, ctx.Err() ; return results
```
</details>

<details><summary>Hint 2 — "start no new calls after a failure"</summary>

The `if runCtx.Err() != nil { return }` check right after receiving an index, *and* the feeder's `select` on `runCtx.Done()`.
</details>

<details><summary>Hint 3 — the test that hangs</summary>

`Map` must return in a few ms for these tests. If one hangs for 5 s, a worker is blocked: usually the feeder tries to send on `indexes` after all workers have exited. Make the feeder `select` on `runCtx.Done()` too.
</details>

<details><summary>Hint 4 — ForEach</summary>

Same pattern as Step 01's `query`, but instead of appending to a slice, call `yield(item)` inside the `rows.Next()` loop and translate its return value.
</details>

## 5. Stretch challenges

1. Replace the feeder+workers with the **semaphore** style: one goroutine per item, throttled by `sem := make(chan struct{}, maxParallel)`. Which test now needs a `runCtx.Err()` check after acquiring the slot?
2. Add `golang.org/x/sync/errgroup` (`go get`) and re-implement `Map` with `errgroup.WithContext` + `SetLimit`. What disappeared? What remained your job (ordering? the first-error rule is built in — why?).
3. Go 1.23+: change `ForEach` to return `iter.Seq2[sales.SoldItem, error]` and use `for item, err := range ...`. `break` now plays the role of `ErrStop`.
4. `MapStream`: take a `<-chan T` instead of a slice.
5. Use `Map` to load the products of an order in parallel in Step 06's service. Measure with a fake repository that sleeps 50 ms.

## 6. Reflect

- Why is firing `go f(item)` for 10 000 items at a database dangerous even though goroutines are cheap? What is actually scarce?
- What bug does `errors.Join`-style "report everything" avoid that "report the first" has, and vice versa?
- Why can the feeder `close(indexes)` safely when Step 07 said *never close a channel with senders*? (Count the senders.)
- A goroutine that is blocked forever on a channel nobody will ever send to: what is that called, and how would you detect it in a test?

## 7. Done when

`go test ./internal/concurrency ./internal/sales -run Step09 -race -count=20` → **ok**.
