# Step 07 — A background job queue (channels & worker pool)

| | |
|---|---|
| **Build** | `Queue` (`internal/jobs/queue.go`), `Worker` (`internal/jobs/worker.go`) |
| **Tests** | `internal/jobs/jobs_test.go` — 17 tests |
| **Run** | `go test ./internal/jobs -run Step07 -race -v` |
| **You'll learn** | Buffered channels as bounded queues, back-pressure, `select`, **the "send on closed channel" trap**, worker pools, `sync/atomic`, `recover`, graceful shutdown with `context` |
| **Time** | ~90–120 min |

## 1. The story

Some work shouldn't happen while the customer waits: invoices, PDFs, warehouse sync. The web request drops a **job** on a queue and returns; background workers pick jobs up. You'll build the queue and the worker pool.

## 2. Concept corner

**A buffered channel *is* a bounded queue.** `make(chan Job, 100)`: sends succeed immediately while there's room, block when it's full; receives block when it's empty. A goroutine blocked on a channel costs almost nothing, so idle workers are free (no polling loops).

**Back-pressure.** An *unbounded* queue turns a slow consumer into an out-of-memory crash. A *bounded* one makes `Enqueue` **wait** when full, slowing the producer to the consumer's pace — the system degrades gracefully instead of dying. `TryEnqueue` is the non-blocking sibling: "full? tell me, I'll answer 503".

**`select` — waiting on several things at once.**

```go
select {
case q.jobs <- job:   return nil              // there was room
case <-q.done:        return ErrQueueClosed   // the queue was closed
case <-ctx.Done():    return ctx.Err()        // the caller gave up
}
```
When several cases are ready, Go picks **at random** — so check "already closed / already cancelled" in a plain `select`-with-`default` *first* if a deterministic answer matters.

**!!! The big Go trap: closing a channel that has senders !!!**
- `close(ch)` while another goroutine is *sending* ⇒ that sender **panics** ("send on closed channel").
- Closing an already-closed channel ⇒ panic.
- The rule of thumb: *only the sender side closes, and only when it knows no one else will send.* A queue with many producers can't know that.

So `Queue.Close()` must **not** close the jobs channel. Keep a separate `done chan struct{}`, closed exactly once with `sync.Once`. Everyone who might block `select`s on it. Consumers prefer a ready job, and after `done` fires they **drain** what's left, then report `ErrQueueClosed`. A test blocks a producer on a full queue, calls `Close()`, and expects `ErrQueueClosed` — not a panic.

**Worker pool.**

```go
var wg sync.WaitGroup
for range w.workers { wg.Add(1); go func() { defer wg.Done(); w.loop(ctx) }() }
wg.Wait()
return ctx.Err()      // nil when the queue closed and drained; context.Canceled on shutdown
```

**Three outcomes of a job:** succeeded, failed, *interrupted by shutdown*. Interrupted ≠ failed. If the handler returns an error **because** `ctx` was cancelled (`ctx.Err() != nil && errors.Is(err, ctx.Err())`) count nothing, otherwise every deploy produces a burst of "failures".

**Poison jobs and panics.** One job that always errors must not stop the pool. One job that *panics* must not crash the program: `recover()` per job (in a deferred function inside the goroutine) and count it as failed.

**Counting from many goroutines.** `n++` on a shared `int` loses updates (and `-race` flags it). Use `atomic.Int64` (`Add`, `Load`).

**Graceful shutdown = cancel the context.** `Run(ctx)` is the whole lifecycle: when `ctx` is cancelled (`signal.NotifyContext` in a real `main`), handlers see it, loops exit, `Run` returns. No `Stop()` method needed.

## 3. Requirements

### `Queue`
- `NewQueue(capacity)`: `< 1` ⇒ error.
- `Enqueue(ctx, job)`: waits while full; `ctx.Err()` if the context ends; `ErrQueueClosed` if closed (or closed while waiting).
- `TryEnqueue(job)`: never waits; `false` if full or closed.
- `Dequeue(ctx)`: waits; `ctx.Err()` if the context ends first; after `Close` returns queued jobs, then `ErrQueueClosed`.
- `Close()`: idempotent; queued jobs are still delivered.
- FIFO order; many producers and consumers at once, every job delivered exactly once.

### `Worker`
- `NewWorker(queue, handler, workers)`: `< 1` ⇒ error.
- `Run(ctx)`: exactly `workers` goroutines running concurrently; every job once; returns `nil` when the queue is closed and drained, `ctx.Err()` when stopped by cancellation.
- Handler error ⇒ `Failed()++`; **panic** ⇒ recovered, `Failed()++`; success ⇒ `Processed()++`; interrupted by shutdown ⇒ neither.
- The context is passed to handlers; an idle `Run` stops promptly when cancelled.

## 4. Hints

<details><summary>Hint 1 — Queue's fields</summary>

`jobs chan Job` (buffered), `done chan struct{}`, `closeOnce sync.Once`. `Close` = `q.closeOnce.Do(func() { close(q.done) })`.
</details>

<details><summary>Hint 2 — Dequeue after close</summary>

```go
select { case j := <-q.jobs: return j, nil; default: }          // a waiting job always wins
select {
case j := <-q.jobs: return j, nil
case <-q.done:
    select { case j := <-q.jobs: return j, nil; default: return Job{}, ErrQueueClosed }   // drain, then closed
case <-ctx.Done(): return Job{}, ctx.Err()
}
```
</details>

<details><summary>Hint 3 — classifying a job's result</summary>

```go
switch {
case err == nil:                                   processed.Add(1)
case ctx.Err() != nil && errors.Is(err, ctx.Err()): // shutdown: count nothing
default:                                           failed.Add(1)
}
```
</details>

<details><summary>Hint 4 — recover per job</summary>

Wrap the handler call in a small method with `defer func() { if r := recover(); r != nil { w.failed.Add(1) } }()`.
</details>

## 5. Stretch challenges

1. **Retry failing jobs** with your Step 08 `Retry`: wrap the `Handler` — the worker doesn't change.
2. **Dispatch by `Job.Type`:** a `Router` handler holding a `map[string]Handler`. Unknown type ⇒ failed job.
3. A **dead-letter queue**: failed jobs (with their error) go to a second queue.
4. Real shutdown: a `main` that uses `signal.NotifyContext(context.Background(), os.Interrupt)`, enqueues some slow jobs, and shows Ctrl+C stopping gracefully.
5. Prove the trap to yourself: change `Close()` to `close(q.jobs)` and watch the "send on closed channel" panic in the tests. (Revert!)

## 6. Reflect

- What does `Run` do if `workers` is 1 and a handler takes ten minutes? How would you notice the queue backing up?
- Producer is an HTTP handler and the queue is full: block (`Enqueue`) or reject (`TryEnqueue`)? What does a caller prefer?
- Jobs here vanish if the process dies. What would a durable queue need (hint: Step 06's outbox idea)?
- Why is `done` a `chan struct{}` rather than a `chan bool`?

## 7. Done when

`go test ./internal/jobs -run Step07 -race -count=10` → **ok**.
