package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"shopflow/internal/domain"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(d *sql.DB) *SQLiteRepository { return &SQLiteRepository{db: d} }

func (r *SQLiteRepository) Place(ctx context.Context, o NewOrder) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op once Commit has succeeded; undoes everything on any early return

	var orderID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, total_cents, status, created_at_utc) VALUES (?, ?, 'Confirmed', ?) RETURNING id`,
		o.CustomerID, o.TotalCents, o.CreatedAt.UTC().Format(time.RFC3339Nano),
	).Scan(&orderID)
	if err != nil {
		return 0, fmt.Errorf("insert order: %w", err)
	}

	for _, line := range o.Lines {
		res, err := tx.ExecContext(ctx,
			`UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?`,
			line.Quantity, line.ProductID, line.Quantity)
		if err != nil {
			return 0, fmt.Errorf("reserve stock of product %d: %w", line.ProductID, err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return 0, fmt.Errorf("reserve stock of product %d: %w", line.ProductID, err)
		} else if n == 0 {
			return 0, &OutOfStockError{ProductID: line.ProductID} // the deferred Rollback undoes the earlier lines
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO order_lines (order_id, product_id, quantity, unit_price_cents) VALUES (?, ?, ?, ?)`,
			orderID, line.ProductID, line.Quantity, line.UnitPriceCents); err != nil {
			return 0, fmt.Errorf("insert order line: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return orderID, nil
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (domain.Order, error) {
	o := domain.Order{ID: id}
	var status, createdAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT customer_id, total_cents, status, created_at_utc FROM orders WHERE id = ?`, id,
	).Scan(&o.CustomerID, &o.TotalCents, &status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Order{}, fmt.Errorf("order %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order %d: %w", id, err)
	}
	o.Status = domain.OrderStatus(status)
	if o.CreatedAt, err = time.Parse(time.RFC3339, createdAt); err != nil {
		return domain.Order{}, fmt.Errorf("order %d: bad timestamp %q: %w", id, createdAt, err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, quantity, unit_price_cents FROM order_lines WHERE order_id = ? ORDER BY id`, id)
	if err != nil {
		return domain.Order{}, fmt.Errorf("get lines of order %d: %w", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		var l domain.OrderLine
		if err := rows.Scan(&l.ProductID, &l.Quantity, &l.UnitPriceCents); err != nil {
			return domain.Order{}, fmt.Errorf("scan order line: %w", err)
		}
		o.Lines = append(o.Lines, l)
	}
	if err := rows.Err(); err != nil {
		return domain.Order{}, fmt.Errorf("iterate order lines: %w", err)
	}
	return o, nil
}

var _ Repository = (*SQLiteRepository)(nil)
