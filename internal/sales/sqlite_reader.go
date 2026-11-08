package sales

import (
	"context"
	"database/sql"

	"shopflow/internal/todo"
)

// SQLiteReader -- STEP 09 (part 2): stream sold items row by row.
//
//	SELECT ol.product_id, ol.quantity
//	FROM order_lines ol JOIN orders o ON o.id = ol.order_id
//	WHERE o.status <> 'Cancelled'
//	ORDER BY ol.id
//
// Requirements:
//   - Every line of every NON-cancelled order, ordered by line id, one row at a time (never collect all rows first).
//   - yield returns ErrStop (errors.Is) -> stop and return nil. Any other yield error -> stop and return it.
//   - Honour ctx: if it is cancelled mid-iteration, return its error (check ctx.Err() inside the loop).
//   - ALWAYS release the connection: `defer rows.Close()` right after QueryContext, so that stopping early, returning an
//     error or being cancelled never leaves the connection checked out of the pool. (A test runs with a pool of ONE
//     connection: a leak freezes the next query.)
//   - After the loop, return rows.Err() -- a false from rows.Next() can mean "something went wrong", not just "done".
type SQLiteReader struct {
	db *sql.DB
}

func NewSQLiteReader(d *sql.DB) *SQLiteReader { return &SQLiteReader{db: d} }

func (r *SQLiteReader) ForEach(ctx context.Context, yield func(SoldItem) error) error {
	return todo.NotImplemented("Step 09: SQLiteReader.ForEach")
}

var _ Reader = (*SQLiteReader)(nil)
