# Step 04 — A caching decorator (and the cache stampede)

| | |
|---|---|
| **Build** | `CachingProductRepository` (`internal/caching/caching_repository.go`) |
| **Tests** | `internal/caching/caching_test.go` — 10 tests |
| **Run** | `go test ./internal/caching -run Step04 -race -v` |
| **You'll learn** | Decorator pattern & Liskov, compile-time interface assertions, `sync.Mutex`, the in-flight-call map (a hand-made *singleflight*), `close(done)` as a broadcast, `select`, `context.WithoutCancel`, the memory model, the race detector |
| **Time** | ~75–100 min — short code, deep ideas |

## 1. The story

Product lookups are hammered: every order validates stock, then prices, then checks credit — each asking for the same products. We want a cache, but we refuse to touch `SQLiteProductRepository` (it works, it's tested, and caching isn't its job).

## 2. Concept corner

**Decorator.** A decorator satisfies the *same interface* as the thing it wraps *and holds one*:

```go
var repo catalog.ProductRepository = caching.NewCachingProductRepository(catalog.NewSQLiteProductRepository(db))
```

Callers just see a `ProductRepository`. You can stack decorators (logging around caching around SQLite) with no class knowing about the others. The line at the bottom of the stub, `var _ catalog.ProductRepository = (*CachingProductRepository)(nil)`, is the idiomatic compile-time check that you really implemented everything.

**Liskov Substitution.** A decorator must be usable anywhere the original is without surprising anyone: same results, `ErrNotFound` still means "not found", cancellation still works.

**The cache stampede.** Naive caching:

```go
if p, ok := cache[id]; ok { return p, nil }       // 1. miss
p, err := inner.GetByID(ctx, id)                   // 2. 50 goroutines all get here before any finishes
cache[id] = p                                      // 3. → 50 database queries for the same row
```

The cache only helps *after* the first call finishes. During that window every concurrent caller piles onto the database. The fix: remember the **call in flight** and let latecomers *wait for it*:

```go
type call struct {
    done    chan struct{}   // closed when the result is ready
    product domain.Product
    err     error
}
// r.inflight map[int64]*call   guarded by r.mu
```

First caller for an id: register a `call`, start the fetch. Everybody else for that id: find the `call` and `<-c.done`. This is exactly what `golang.org/x/sync/singleflight` does — you're building a tiny version.

**`close(done)` is a broadcast.** Receiving from a closed channel returns immediately, *for every receiver*. So one `close(c.done)` wakes all 49 waiters. (Sending a value would wake only one.)

**The memory model, briefly.** You write `c.product, c.err = p, err` and *then* `close(c.done)`; waiters read those fields *after* `<-c.done`. Go guarantees a close happens-before a receive that observes it, so this is race-free **without** a lock around the fields. The race detector (`-race`) will confirm — or scream if you get the order wrong.

**Whose context runs the shared fetch?** If the *first* caller's context drives the fetch and that caller cancels, everybody waiting on it fails too. So run the fetch with `context.WithoutCancel(ctx)` (Go 1.21+: keeps values, drops cancellation) and let each caller stop *waiting* individually:

```go
select {
case <-c.done:       return c.product, c.err
case <-ctx.Done():   return domain.Product{}, ctx.Err()    // this caller gives up; the fetch carries on
}
```

**Don't cache bad news.** A cached error or "not found" would keep answering wrongly forever. Only successful results go in the cache; the in-flight entry is removed either way.

## 3. Requirements

1. `GetByID` caches **found** products per id; a second call doesn't reach the inner repository.
2. "Not found" is **not** cached; other errors are **not** cached.
3. **Stampede protection:** 50 simultaneous callers for the same uncached id ⇒ the inner repository is called **exactly once**; all get the same product.
4. A caller whose context is cancelled gets `ctx.Err()` promptly; others still get the result; the inner call still happens once.
5. `UpdateStock` calls the inner repository, then evicts that id, and returns the inner error.
6. `List`, `ListByCategory`, `Add` delegate.
7. Safe for concurrent use — the tests run a mixed read/update workload under the race detector.

## 4. Hints

<details><summary>Hint 1 — the struct</summary>

```go
mu       sync.Mutex
cache    map[int64]domain.Product
inflight map[int64]*call
```
Initialise both maps in the constructor (writing to a nil map panics).
</details>

<details><summary>Hint 2 — GetByID's skeleton</summary>

Lock; return from `cache` if present; else look in `inflight` — if absent create a `call`, store it and `go r.fetch(ctx, id, c)`; unlock; then the `select` above.
</details>

<details><summary>Hint 3 — fetch</summary>

Call the inner repository with `context.WithoutCancel(ctx)`; lock; `delete(r.inflight, id)`; on success `r.cache[id] = p`; unlock; set `c.product, c.err`; **then** `close(c.done)`.
</details>

<details><summary>Hint 4 — why a test hangs for 5 seconds</summary>

`ACallerThatCancels…` waits for a value that never comes if a cancelled caller's context also cancelled the shared fetch (or you don't `select` on `ctx.Done()`).
</details>

## 5. Stretch challenges

1. **Expiry:** entries older than 30 s are refetched. Inject `clock.Clock` and test with `FakeClock`.
2. Compare with the real `singleflight.Group` (`go get golang.org/x/sync`): replace your in-flight map with it. What did it buy you? What did it *not* do (hint: the cache itself, per-caller cancellation)?
3. A panic inside `inner.GetByID` currently kills the program from the fetch goroutine. Recover it in `fetch`, turn it into an error for the waiters, and make sure `done` is *always* closed (hint: `defer`).
4. After `UpdateStock`, an *in-flight* older fetch may finish and put **stale** data in the cache. Construct that interleaving. How would a version counter prevent it?
5. Add a `LoggingProductRepository` decorator and stack it above and below the cache. What does each placement show?

## 6. Reflect

- Why is "check the cache, then call inner, then store" wrong even if every map access is under the mutex?
- Why must `close(c.done)` come *after* the result fields are written?
- Why does the fetch goroutine use `WithoutCancel`, and what would the trade-off be of *never* cancelling a shared fetch?
- Embedding (`struct{ catalog.ProductRepository }`) would give you all five methods for free. Why is a hand-written decorator better here? (The *tests* use embedding — look at `countingProducts` — why is it fine there?)

## 7. Done when

`go test ./internal/caching -run Step04 -race -count=5` → **ok**.
