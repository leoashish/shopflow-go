# Step 06 — Place an order (transactions + orchestration)

| | |
|---|---|
| **Build** | `SQLiteRepository` (`internal/orders/sqlite_repository.go`), `Service.PlaceOrder` (`internal/orders/service.go`) |
| **Tests** | `internal/orders/orders_test.go` — 13 tests |
| **Run** | `go test ./internal/orders -run Step06 -race -v` |
| **You'll learn** | `BeginTx`, the `defer tx.Rollback()` idiom, atomic guarded updates vs read-modify-write races, typed errors with `errors.As`, `BEGIN IMMEDIATE` + busy timeout, an application service with a dependency struct |
| **Needs** | Your solutions to **Steps 1, 2, 3 and 5** (or `bash scripts/apply-solution.sh N` for any you skipped) |
| **Time** | ~90–120 min |

## 1. The story

The moment of truth: a customer presses "Buy". We must validate, price, record the order, take the items out of stock and tell everybody — and it must **never** end up half-done (stock gone but no order; order but no stock), even when several customers click at the same instant.

## 2. Concept corner

**Transactions.** Several statements as one unit: all happen or none. In `database/sql`:

```go
tx, err := r.db.BeginTx(ctx, nil)
if err != nil { return 0, fmt.Errorf("begin: %w", err) }
defer tx.Rollback()            // the idiom: harmless after Commit, undoes everything on ANY early return
// ... tx.QueryRowContext / tx.ExecContext (NOT r.db.…) ...
if err := tx.Commit(); err != nil { ... }
```

That `defer tx.Rollback()` is the beautiful part: an error halfway through just returns, and the deferred rollback undoes it all — including lines that already succeeded. Everything inside the transaction must go through `tx`; a stray `r.db.ExecContext` runs *outside* it (and a test notices the missing rollback).

**Race conditions and the guarded update.** The obvious stock logic:

```go
stock := readStock(id)                 // both buyers read: "1 left"
if stock >= qty { writeStock(id, stock-qty) }   // both write → oversold!
```
Between your read and your write someone else can sneak in. Make check-and-change **one atomic statement**:

```sql
UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?
```
Zero rows affected (`res.RowsAffected()`) ⇒ the stock wasn't there ⇒ return an error and let the deferred rollback clean up. The database serialises concurrent writers, so exactly one buyer wins the last unit.

**Validation *and* the guard.** The chain (Step 3) gives friendly early errors and avoids wasted work, but it's a *read*; by the time you write, things may have changed. The guarded `UPDATE` is the real safety net. The service turns the guard's `*OutOfStockError` into the same `INSUFFICIENT_STOCK` result validation would give.

**Typed errors and `errors.As`.** Sentinels (`ErrNotFound`) say *what*; typed errors also carry *data*:

```go
type OutOfStockError struct{ ProductID int64 }
func (e *OutOfStockError) Error() string { ... }

var oos *orders.OutOfStockError
if errors.As(err, &oos) { ... oos.ProductID ... }      // finds it even if wrapped with %w
```

**SQLite and concurrent writers.** `internal/db/db.go` opens with `_txlock=immediate` (every `BeginTx` becomes `BEGIN IMMEDIATE`: the write lock is taken *up front*), `_busy_timeout=5000` (wait up to 5 s for a lock instead of failing instantly) and WAL mode (readers don't block the writer). Result: six concurrent buyers *queue up* instead of failing with "database is locked". Open `db.go` and read its comment.

**`(Result, error)` — business vs infrastructure again.** `PlaceOrder` returns `Result` (either the order or the business errors) *and* a separate `error` for infrastructure failures, exactly like `validation.Handler`.

**Dependencies via a struct.** `Deps` has seven fields — all interfaces or small collaborators. Go has no DI container; you wire things by hand in `main` (or in a test harness, see `newHarness`). Notice `Publisher` is a one-method interface *defined in the `orders` package* — `*events.Publisher` satisfies it without knowing it exists.

**Order of side effects.** Publish **after** the commit. If you publish first and the commit fails, you've announced an order that doesn't exist. The inverse problem (committed but the notification failed) is why the service ignores publish errors; the robust answer is the *outbox pattern* (Stretch 3).

## 3. Requirements

### `SQLiteRepository.Place(ctx, order)` — in **one transaction**
1. `INSERT` the order (`status 'Confirmed'`, `created_at_utc = o.CreatedAt.UTC().Format(time.RFC3339Nano)`), getting the id back with `RETURNING id`.
2. For each line the guarded `UPDATE`; zero rows ⇒ `return 0, &OutOfStockError{ProductID: …}`.
3. `INSERT` the line (snapshot unit price).
4. `Commit`; return the id.
An out-of-stock line rolls back **everything**. A cancelled context ⇒ an error wrapping `context.Canceled`, nothing saved.

### `SQLiteRepository.GetByID(ctx, id)`
Header + lines ordered by line id; no such order ⇒ wraps `domain.ErrNotFound`. Parse the timestamp with `time.Parse(time.RFC3339, s)` (it reads both seed rows and the nanosecond format).

### `Service.PlaceOrder(ctx, req)`
1. Run `Validate`. Error ⇒ `(Result{}, err)` (wrapped); validation errors ⇒ `(Result{Errors: errs}, nil)`, nothing touched.
2. Load the customer and products; price the cart with `Pricing`.
3. Build the `NewOrder`: line unit price = the product's **current price** (undiscounted); total = `breakdown.TotalCents`; `CreatedAt = Clock.Now()`.
4. `Orders.Place`; `*OutOfStockError` ⇒ `Result{Errors: [one CodeInsufficientStock]}`; other errors ⇒ returned (wrapped).
5. Reload the order and publish `events.OrderPlaced`. **Ignore** a publish error — the order is committed.
6. `Result{Order: &order}`.

## 4. What the tests pin down

Saved rows and snapshot prices; stock reduced (checked with raw SQL, independent of your Step 1 code); **full rollback** (stock, order and line counts unchanged); a worked pricing example (Gold customer, 10 cables + 1 keyboard ⇒ **16191** cents); inactive/stock/credit failures leave the database untouched; an infrastructure error stays an `error`; a broken notifier doesn't fail the order; **six goroutines racing for one remaining unit ⇒ exactly one winner, stock 0, never negative**; cancellation.

## 5. Hints

<details><summary>Hint 1 — the transaction skeleton</summary>

```go
tx, err := r.db.BeginTx(ctx, nil)
...
defer tx.Rollback()
var id int64
err = tx.QueryRowContext(ctx, `INSERT ... RETURNING id`, ...).Scan(&id)
```
</details>

<details><summary>Hint 2 — detecting "no stock"</summary>

```go
res, err := tx.ExecContext(ctx, `UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?`, q, id, q)
n, err := res.RowsAffected()
if n == 0 { return 0, &OutOfStockError{ProductID: id} }
```
`?` appears twice with the same value: pass the value twice.
</details>

<details><summary>Hint 3 — GetByID with two queries</summary>

Header with `QueryRowContext(...).Scan(...)`; lines with `QueryContext` + `defer rows.Close()` + `rows.Err()` (Step 01 again).
</details>

<details><summary>Hint 4 — the service is a recipe</summary>

Follow the numbered list in the doc comment on `Service` line by line. Resist adding extra logic. `errors.As(err, &outOfStock)` where `var outOfStock *OutOfStockError`.
</details>

## 6. Stretch challenges

1. `Cancel(ctx, orderID)`: set the status to `Cancelled` **and** put the stock back, in one transaction. Make it idempotent.
2. Load the products in `PlaceOrder` concurrently with Step 09's `concurrency.Map`. Worth it for 3 lines? For 300?
3. **Outbox pattern:** write the "order placed" event into an `outbox` table *inside the same transaction*, and have a Step 07 worker publish and mark rows done. Which failure does this eliminate?
4. Add an **idempotency key** (`UNIQUE` column on orders): submitting the same request twice (a network retry!) creates one order. Where is the race this time?

## 7. Reflect

- Delete `AND stock >= ?` from the SQL (don't commit that!) and re-run the race test. What happens, and why doesn't the *validation* step save you?
- What exactly does `defer tx.Rollback()` do *after* a successful `Commit`?
- Why does `Place` live in the repository instead of the service calling a `ProductRepository.UpdateStock` for each line?
- What would you have to change to run this on a database where transactions span several machines?

## 8. Done when

`go test ./internal/orders -run Step06 -race -count=10` → **ok** (run it repeatedly; concurrency tests are only trustworthy if they're stable).
