package orders

import (
	"context"
	"database/sql"

	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// SQLiteRepository -- STEP 06: the transactional heart of the app. (It needs *sql.DB, not db.DBTX: only a *sql.DB can BeginTx.)
//
// Place, inside ONE transaction (tx, err := r.db.BeginTx(ctx, nil)):
//  1. INSERT the order -- status 'Confirmed', created_at_utc = o.CreatedAt.UTC().Format(time.RFC3339Nano) -- and get
//     its id back (hint: "INSERT ... RETURNING id" with tx.QueryRowContext(...).Scan(&id)).
//  2. For every line, reserve stock with a GUARDED update:
//     UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?
//     If res.RowsAffected() is 0, return &OutOfStockError{ProductID: ...}.
//  3. INSERT the order line (with the snapshot unit price).
//  4. tx.Commit().
//
// The classic Go idiom: right after BeginTx write `defer tx.Rollback()`. After a successful Commit the rollback is a
// harmless no-op; on ANY early return it undoes everything -- including lines that already succeeded.
//
// Why a guarded UPDATE instead of "SELECT stock; check in Go; UPDATE"? Between your read and your write another request
// can sneak in (a race condition). The WHERE clause makes check-and-change ONE atomic step.
//
// Use tx.QueryRowContext / tx.ExecContext (not r.db...) for everything inside the transaction.
//
// GetByID: load the order header and its lines (ordered by line id). No such order -> an error wrapping domain.ErrNotFound.
// Status is stored as text; the timestamp as RFC 3339 text (time.Parse(time.RFC3339, s) reads both seed and new rows).
type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(d *sql.DB) *SQLiteRepository { return &SQLiteRepository{db: d} }

func (r *SQLiteRepository) Place(ctx context.Context, o NewOrder) (int64, error) {
	return 0, todo.NotImplemented("Step 06: SQLiteRepository.Place")
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (domain.Order, error) {
	return domain.Order{}, todo.NotImplemented("Step 06: SQLiteRepository.GetByID")
}

var _ Repository = (*SQLiteRepository)(nil)
