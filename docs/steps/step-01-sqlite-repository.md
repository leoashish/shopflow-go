# Step 01 — A SQLite repository with `database/sql`

| | |
|---|---|
| **Build** | `SQLiteProductRepository` — `internal/catalog/sqlite_product_repository.go` |
| **Tests** | `internal/catalog/catalog_test.go` — 14 tests |
| **Run** | `go test ./internal/catalog -run Step01 -v` |
| **You'll learn** | `database/sql` (pool, `Context` methods, `Rows` lifecycle), sentinel errors & wrapping, `errors.Is`, "accept interfaces, return structs", consumer-defined interfaces |
| **Time** | ~60–90 min |

## 1. The story

The shop needs to read and update its product catalogue. Every other feature (pricing, validation, orders…) asks questions like *"what does product 12 cost, and how many are left?"* — and none of them should know the answer comes from SQLite. That knowledge lives in exactly one place: the **repository**.

## 2. Concept corner

**There is no `async/await` in Go.** Go code *blocks*, and goroutines (cheap, thousands at a time) make blocking fine: while one goroutine waits for the database, the runtime runs others. So `repo.GetByID(ctx, 1)` just returns the product — no `Task`, no `await`. Concurrency is something *callers* opt into with `go`; you'll start doing that from Step 4.

**`*sql.DB` is a pool, not a connection.** It is safe for concurrent use and meant to be created once and shared. Each query borrows a connection and returns it when done. (Compare .NET, where you opened a connection per operation yourself.)

**Always the `…Context` methods.** `QueryRowContext(ctx, …)`, `QueryContext(ctx, …)`, `ExecContext(ctx, …)` — never `Query`/`Exec`. The context is how callers cancel a slow query or give it a deadline. Pass the `ctx` you were given; never invent `context.Background()` inside a repository. A test calls every method with an already-cancelled context.

**`Rows` must be closed.** `QueryContext` returns `*sql.Rows` which *holds a connection* until closed:

```go
rows, err := r.db.QueryContext(ctx, query, args...)
if err != nil { return nil, fmt.Errorf("query products: %w", err) }
defer rows.Close()                 // ALWAYS, right here — runs however the function exits
for rows.Next() { ... rows.Scan(...) ... }
if err := rows.Err(); err != nil { ... }   // false from Next() can mean "error", not just "no more rows"
```

Forget `Close` on an early return and the connection leaks; with enough leaks the pool is empty and *everything* hangs. A test runs with a pool of **one** connection to catch exactly this.

**Errors are values.** "Not found" is not an exception; it is a sentinel value, `domain.ErrNotFound`. Translate the driver's `sql.ErrNoRows` into it, and **wrap** with `%w` so callers can still ask `errors.Is(err, domain.ErrNotFound)` *and* get context in the message:

```go
if errors.Is(err, sql.ErrNoRows) {
    return domain.Product{}, fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
}
```

**Accept interfaces, return structs.** `NewSQLiteProductRepository` takes a `db.DBTX` (a 3-method interface that both `*sql.DB` and `*sql.Tx` satisfy) and returns a concrete `*SQLiteProductRepository`. That is Dependency Inversion Go-style — and `catalog.ProductRepository` (the interface the rest of the app uses) lives next to its *consumers'* needs. A line like `var _ ProductRepository = (*SQLiteProductRepository)(nil)` is a compile-time proof that your type really satisfies it.

**Parameters, not string building.** `?` placeholders keep input out of the SQL text. One test tries an injection attack on you.

## 3. Requirements

Implement the five methods of `catalog.ProductRepository` for SQLite:

| Method | Behaviour |
|---|---|
| `GetByID(ctx, id)` | The product, or an error satisfying `errors.Is(err, domain.ErrNotFound)`. |
| `List(ctx)` | Every product ordered by id. |
| `ListByCategory(ctx, category)` | Exact (case-sensitive) match on `category`, ordered by id; an empty slice if none. Safe against SQL injection. |
| `Add(ctx, p)` | Inserts the product (ignore `p.ID`), returns the **new id**. A duplicate `sku` makes SQLite fail — return that error (it must **not** look like `ErrNotFound`). |
| `UpdateStock(ctx, id, newStock)` | Updates `stock`. No such product ⇒ `ErrNotFound`. A negative stock violates the table's `CHECK` constraint ⇒ return the driver's error. |

Cross-cutting: every method returns an error wrapping `context.Canceled` when given a cancelled context; calls from many goroutines at once all work; **no method leaks a connection**.

## 4. What the tests pin down

All six columns mapped; `ErrNotFound` via `errors.Is`; ordering; the injection attempt returns nothing; `Add` returns id 21 (the seed has 20) and round-trips; unique and CHECK constraints surface as errors; cancellation; 20 goroutines reading in parallel (run with `-race`); and the single-connection leak test.

## 5. Hints (open one at a time)

<details><summary>Hint 1 — the shape</summary>

Open `SQLiteCustomerRepository.GetByID`. Yours is the same code with different columns: `QueryRowContext(...).Scan(...)`, then translate `sql.ErrNoRows`.
</details>

<details><summary>Hint 2 — one scan function for Row and Rows</summary>

Both `*sql.Row` and `*sql.Rows` have `Scan(dest ...any) error`. Declare `type scanner interface{ Scan(dest ...any) error }` and write `func scanProduct(s scanner) (domain.Product, error)` once.
</details>

<details><summary>Hint 3 — a shared list helper</summary>

`List` and `ListByCategory` differ only in the SQL and args. A private `query(ctx, sql string, args ...any) ([]domain.Product, error)` holds the `defer rows.Close()` / `rows.Next()` / `rows.Err()` dance once.
</details>

<details><summary>Hint 4 — getting the new id</summary>

SQLite 3.35+ supports `INSERT ... RETURNING id`. Run it with `QueryRowContext(...).Scan(&id)`.
</details>

<details><summary>Hint 5 — "no such product" for an UPDATE</summary>

`res, err := r.db.ExecContext(...)` then `res.RowsAffected()`; zero means nothing matched.
</details>

## 6. Stretch challenges

1. Add `Delete(ctx, id)` to the interface. What does the schema say about order lines that reference the product?
2. `SearchByName(ctx, text)` using `LIKE ?`. Where do the `%` wildcards go — in the SQL or in the parameter value? Why does it matter for safety?
3. **Keyset pagination:** `ListAfter(ctx, afterID, limit)` using `WHERE id > ? ORDER BY id LIMIT ?`. Why is that more robust than `OFFSET` when rows are inserted between pages?
4. `GetByIDs(ctx, ids []int64)` building `IN (?,?,?)` placeholders safely.
5. Use `sql.NullString`/`sql.NullInt64` for a nullable column (add one to the schema).

## 7. Reflect

- Why does the repository take `db.DBTX` rather than `*sql.DB`? What does that enable in Step 6?
- What exactly goes wrong if `List` forgets `defer rows.Close()` but always loops to the end? And if it returns early on a scan error?
- Why return `domain.ErrNotFound` instead of `(nil, nil)` for a missing product? What does the caller gain?
- Which parts of this code would change for PostgreSQL? Which would not?

## 8. Done when

`go test ./internal/catalog -run Step01 -race` → **ok**. Then `go vet ./...`, `gofmt -l .`, and compare with `_solutions/step01/…`.
