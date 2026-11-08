# Step 08 — Retry with backoff & a circuit breaker (State pattern)

| | |
|---|---|
| **Build** | `Retry` (`internal/resilience/retry.go`) and the three states in `states.go` |
| **Given** | `Breaker` (the State pattern's *context*, `breaker.go`), `State`, `ErrOpen`, `clock.Clock`, `clock.Sleeper` |
| **Tests** | `internal/resilience/resilience_test.go` — 20 tests |
| **Run** | `go test ./internal/resilience -run Step08 -race -v` |
| **You'll learn** | Exponential backoff, context-aware sleeping, State pattern with interfaces, thread-safe state machines, the `panicked`-flag defer idiom, testing time without sleeping |
| **Time** | ~90–120 min |

## 1. The story

Our order service calls other systems: payment, shipping, email. Networks hiccup. Two defences:

- **Retry** transient failures — waiting longer each time, or you hammer a struggling service.
- **Circuit breaker** — when a service is clearly down, *stop calling it for a while* instead of letting every request wait for a timeout.

## 2. Concept corner

**Exponential backoff.** After failed attempt *n*, wait `BaseDelay × 2^(n−1)` — 1 s, 2 s, 4 s, 8 s… — capped at `MaxDelay`. (Real systems add **jitter**, a random fraction, so thousands of clients don't retry in lockstep: Stretch 1.)

**What must not be retried:** cancellation (the caller gave up), permanent errors (`ShouldRetry` says so), and — in the composition test — an open circuit.

**Context-aware sleeping.** `time.Sleep(d)` can't be interrupted. `clock.SystemSleeper` (given — read it) waits on a `time.Timer` *and* `ctx.Done()`:

```go
timer := time.NewTimer(d); defer timer.Stop()
select { case <-timer.C: return nil; case <-ctx.Done(): return ctx.Err() }
```
(`time.After` in a loop leaks a timer per call until it fires; `NewTimer` + `Stop` doesn't.) `Retry` sleeps through the injected `Sleeper`, so a test's `RecordingSleeper` records the requested delays and returns instantly. A test that "waits 30 seconds" runs in microseconds and can't flake.

**Return the error you got.** After the last attempt, return the *same* error value — wrapping it ("giving up after 3 attempts: …") would break callers comparing identity, and a test checks `err == boom`. (Wrapping with `%w` is usually *fine* in Go, `errors.Is` still works — but know what you promised.)

**Overflow.** `2^50 × 24 h` doesn't fit in the `int64` inside `time.Duration`. Do the multiplication in `float64` (`math.Pow`, exact for powers of two), compare with the cap, then convert back.

**State pattern with interfaces.** Instead of `if state == open … else if state == halfOpen …` spread everywhere, each state is a *type* that knows how to behave:

```
          failureThreshold consecutive failures
   ┌────────┐ ───────────────────────────────▶ ┌──────┐
   │ Closed │                                  │ Open │ ◀──────────────┐
   └────────┘ ◀─────────────┐                  └──────┘                │
        ▲   success resets    │ trial succeeds     │ openFor elapsed     │ trial fails
        └─ the failure count  │                    ▼ (on the next call)  │
                          ┌───┴──────┐ ◀────────────────────────────────┘
                          │ HalfOpen │   exactly ONE trial call allowed
                          └──────────┘
```
Each state owns only the data it needs: *closed* counts consecutive failures; *open* remembers when it opened; *half-open* remembers whether the trial is running. `Breaker` (given — read it carefully) delegates to the current state, and states ask it to `transitionTo(...)` the next.

**Mutex discipline.** `Breaker` holds its mutex while calling state methods but **not** while your function runs (a slow call must not freeze every other caller). So state methods must be quick, non-blocking and must *never lock the mutex themselves* (Go's `sync.Mutex` is not reentrant — locking twice deadlocks).

**The `panicked` flag.** In the given `Execute`:

```go
panicked := true
defer func() { if panicked { b.record(ctx, errors.New("panic")) } }()
err := fn(ctx)
panicked = false
```
If `fn` panics, the deferred function still runs, records a failure (otherwise a half-open trial slot would stay taken forever) and the panic carries on, because we never called `recover`. A neat trick worth stealing.

**Cancellation is not evidence.** If the caller cancels, we learned nothing about the dependency's health: neither success nor failure. A cancelled *trial* must free the half-open slot, or the breaker would stay stuck.

## 3. Requirements

### `Retry`
- `NewRetry(opts)`: `MaxAttempts < 1` ⇒ error. Defaults: `MaxDelay` 30 s, `ShouldRetry` = always, `Sleeper` = `clock.SystemSleeper{}`.
- `Do(ctx, fn)`: check `ctx.Err()` before every attempt (cancelled ⇒ return it, `fn` not called). Call `fn(ctx)`. Success ⇒ `nil`.
- After failed attempt *n*, if attempts remain **and** `ShouldRetry(err)`: `Sleeper.Sleep(ctx, min(BaseDelay × 2^(n−1), MaxDelay))`; if the sleep returns an error (cancelled), return it.
- Out of attempts, or `ShouldRetry` false ⇒ return the error **as it is** (same value). No sleep after the last attempt.

### The three states
| State | `tryEnter` | `onSuccess` | `onFailure` | `onAborted` |
|---|---|---|---|---|
| **closed** | always `true` | reset the count to 0 | count++; at `b.failureThreshold` → `transitionTo(&openState{openedAt: b.clock.Now()})` | nothing |
| **open** (`openedAt`) | if `b.clock.Now().Sub(openedAt) < b.openFor` → `false`; else `transitionTo(&halfOpenState{})` and return *its* `tryEnter` | nothing | nothing | nothing |
| **half-open** | `true` for the first caller (remember a trial runs), `false` for everyone else | → `&closedState{}` | → `&openState{openedAt: now}` | free the trial slot (stay half-open) |

(`open`'s three "nothing"s exist because a slow call that started *before* the circuit opened may report in late.)

## 4. What the tests pin down

Retry: no sleep on first-try success; delays `[1 s, 2 s]` for two failures; the very same error value after exhaustion; `ShouldRetry`; the cap (`1, 2, 3, 3`); overflow safety (24 h base, 50 attempts); cancellation inside `fn` and between attempts and before starting.
Breaker: opens at the threshold then rejects *without calling the function*; only **consecutive** failures count; stays open until the duration passes (9 s vs 10 s); trial success closes (failure count restarts); trial failure re-opens with a **fresh** timer; only one trial at a time; a cancelled trial frees the slot; cancellation never counts as failure.
Composition: `retry( breaker( flaky ) )` stops retrying when the circuit opens.

## 5. Hints

<details><summary>Hint 1 — the delay</summary>

```go
d := float64(o.BaseDelay) * math.Pow(2, float64(min(failedAttempt-1, 62)))
if d > float64(o.MaxDelay) { return o.MaxDelay }
return time.Duration(d)
```
</details>

<details><summary>Hint 2 — Do's loop</summary>

`for attempt := 1; ; attempt++ { if err := ctx.Err(); err != nil { return err }; err := fn(ctx); if err == nil { return nil }; if attempt >= max || !shouldRetry(err) { return err }; if err := sleeper.Sleep(ctx, delay); err != nil { return err } }`
</details>

<details><summary>Hint 3 — openState.tryEnter</summary>

```go
if b.clock.Now().Sub(s.openedAt) < b.openFor { return false }
half := &halfOpenState{}
b.transitionTo(half)
return half.tryEnter(b)      // the caller who noticed becomes the trial call
```
</details>

<details><summary>Hint 4 — half-open</summary>

One `bool trialInFlight`. `tryEnter`: `if s.trialInFlight { return false }; s.trialInFlight = true; return true`.
</details>

## 6. Stretch challenges

1. **Full jitter:** wait a random duration in `[0, computed]`. Inject a `func() float64` so tests stay deterministic.
2. A **bulkhead** capping concurrent calls to a dependency (a buffered channel as a semaphore, or Step 09).
3. Make the breaker report state changes through a callback or Step 05's publisher — a natural place for alerting.
4. Switch "N consecutive failures" to "≥ 50 % of the last 20 calls". Which state type changes? Which don't? (That is the State pattern paying off.)
5. Compare with a real library such as `sony/gobreaker`. Which of its concepts did you just build?

## 7. Reflect

- Why is `open → half-open` triggered by the *next call* rather than by a timer goroutine?
- Retry *outside* the breaker, or breaker *outside* the retry? Swap them in the composition test and describe what changes.
- What would happen if a state method called `b.mu.Lock()`?
- Why does `Retry` return `ctx.Err()` from the failed sleep rather than the call's last error?

## 8. Done when

`go test ./internal/resilience -run Step08 -race` → **ok**.
