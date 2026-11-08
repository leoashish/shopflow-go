# Step 05 — Events & notifications (Observer + Factory)

| | |
|---|---|
| **Build** | `Publisher` (`internal/events/publisher.go`), `Registry` (`internal/notify/registry.go`), `ConfirmationNotifier` (`internal/notify/confirmation.go`) |
| **Tests** | `internal/events/events_test.go` (8) + `internal/notify/notify_test.go` (7) — 15 tests |
| **Run** | `go test ./internal/events ./internal/notify -run Step05 -race -v` |
| **You'll learn** | Observer pattern, your first goroutines, `sync.WaitGroup`, per-index result slots instead of locks, `errors.Join`, `recover`, copy-on-write + `RWMutex`, registry-style Factory, fail-fast construction |
| **Time** | ~75–100 min |

## 1. The story

When an order is placed, several things should happen: confirm by email and SMS, maybe update analytics later, maybe tell the warehouse. The code that *places* the order shouldn't know about any of them — otherwise every new reaction means editing order logic.

## 2. Concept corner

**Observer.** A *subject* (`Publisher`) keeps a list of *observers* (`Handler`) and tells all of them when something happens. The subject doesn't know what observers do; observers don't know each other. `Subscribe` returns an **unsubscribe function** — the Go idiom for what C# does with `IDisposable`.

**Running handlers concurrently.** One goroutine per handler, then wait for all:

```go
errs := make([]error, len(snapshot))        // one slot per handler
var wg sync.WaitGroup
for i, s := range snapshot {
    wg.Add(1)
    go func() {                              // Go 1.22: i and s are fresh per iteration — safe to capture
        defer wg.Done()
        errs[i] = s.handler.Handle(ctx, e)
    }()
}
wg.Wait()
return errors.Join(errs...)                  // nil if every entry is nil
```

**Why a slice of slots instead of a mutex?** Each goroutine writes only *its own* element; distinct elements are distinct memory locations, so there is no race, and no lock to forget. After `wg.Wait()` all writes are visible (Wait happens-after every `Done`). Collecting into a slice *by index* also makes the joined error come out in subscription order, so results are deterministic.

**`errors.Join`** (Go 1.20) combines errors into one that still answers `errors.Is(err, errA) && errors.Is(err, errB)`. The test checks that *both* failures are reachable — the Go counterpart of .NET's `Task.WhenAll` swallowing all but the first exception.

**Panics in goroutines are fatal — unless recovered.** An unrecovered panic in *any* goroutine kills the whole program. Third-party handlers can panic, so each goroutine does:

```go
defer func() { if r := recover(); r != nil { errs[i] = fmt.Errorf("event handler panicked: %v", r) } }()
```
`recover` only works inside a deferred function in the panicking goroutine — that is why it lives *inside* the `go func()`.

**Never hold a lock while running code you don't control.** If `Publish` kept its mutex locked during handlers, a handler that calls `Subscribe` would deadlock. So: lock, **take a snapshot** of the subscriber slice, unlock, then call handlers. To keep old snapshots valid, never modify the slice in place — **copy-on-write**: build a *new* slice on every Subscribe/unsubscribe (`slices.Clone`).

**Make "unsubscribe" safe to call twice.** `sync.Once` inside the returned closure.

**Factory as a registry.** A classic factory is a `switch name { case "email": return &Email{} … }`; every new channel edits it (violates Open/Closed). `Registry` is *given* the channels and indexes them by `Name()`; a new channel is a new type plus one registration. **Fail fast:** `NewConfirmationNotifier` resolves its channel names *in the constructor* so a typo breaks at startup, not at 3 a.m. on the first order.

## 3. Requirements

### `Publisher`
1. `Subscribe(h)` returns `unsubscribe`; calling it twice is harmless.
2. `Publish` runs every current subscriber **concurrently** (a test makes two handlers wait for each other).
3. A failing handler never stops the others; return `errors.Join` of all failures **in subscription order**; all fine ⇒ `nil`; no subscribers ⇒ `nil`.
4. A **panicking** handler is recovered and becomes an error; the others still run.
5. The context is passed to handlers.
6. Subscribing from inside a handler during a `Publish` must not deadlock, and the new handler only sees *later* events.

### `Registry`
`NewRegistry(channels...)`; `Get(name)` is case-insensitive; unknown ⇒ error wrapping `ErrUnknownChannel` (include the name); duplicate names (ignoring case) ⇒ error wrapping `ErrDuplicateChannel`.

### `ConfirmationNotifier`
- `NewConfirmationNotifier(customers, registry, names)` resolves all names immediately (unknown ⇒ that error).
- `Handle(ctx, e)`: load the customer (any error returned, wrapped with `%w`, nothing sent), then send **one** `Notification` through each channel in order:
  - `Recipient` = customer email
  - `Subject` = `Order #77 confirmed`
  - `Body` = `Hi Aarav, your order #77 totalling $161.91 has been confirmed.` (`money.Format`)
- A failing `Send` ⇒ return that error (wrapped).

## 4. Hints

<details><summary>Hint 1 — the subscription list</summary>

`type subscription struct{ id int; handler Handler }`; `subs []subscription` under an `RWMutex`; Subscribe: `p.subs = append(slices.Clone(p.subs), subscription{...})`. Unsubscribe: `slices.DeleteFunc(slices.Clone(p.subs), func(s subscription) bool { return s.id == id })`.
</details>

<details><summary>Hint 2 — Publish</summary>

`RLock`, copy the slice header into `snapshot`, `RUnlock`, then the goroutine loop above. Put `defer wg.Done()` *first* and the `recover` defer second so both run.
</details>

<details><summary>Hint 3 — registry keys</summary>

Store channels in a `map[string]Channel` keyed by `strings.ToLower(c.Name())`; a second insert with the same key is the duplicate.
</details>

<details><summary>Hint 4 — wrapping a sentinel with context</summary>

`return nil, fmt.Errorf("%w: %q", ErrUnknownChannel, name)` — `errors.Is` still matches, and the message names the culprit.
</details>

## 5. Stretch challenges

1. Make events generic: `Publisher[E any]` with `Handler[E]` (Go generics), then add an `OrderCancelled` event type without touching `Publisher`.
2. Give each handler a **timeout**: run it with `context.WithTimeout`. What should `Publish` return when one handler is merely slow?
3. Make delivery asynchronous: `Publish` only enqueues onto Step 07's job queue and returns. What new failure appears (events lost on a crash)? Read about the **outbox pattern**.
4. Write a real `FileChannel` that appends to a file. Which existing types did you have to change? (If "none" — the design worked.)

## 6. Reflect

- `Service.PlaceOrder` (Step 06) deliberately *ignores* what `Publish` returns. Why is that the right split of responsibility between the two?
- What would happen if two handlers appended to one shared `[]error` with no lock? How would `-race` tell you?
- Observer via interfaces vs. a Go channel per subscriber: when would you pick channels?

## 7. Done when

`go test ./internal/events ./internal/notify -run Step05 -race` → **ok**.
