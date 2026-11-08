package sales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SQLiteReader struct {
	db *sql.DB
}

func NewSQLiteReader(d *sql.DB) *SQLiteReader { return &SQLiteReader{db: d} }

func (r *SQLiteReader) ForEach(ctx context.Context, yield func(SoldItem) error) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ol.product_id, ol.quantity
		FROM order_lines ol JOIN orders o ON o.id = ol.order_id
		WHERE o.status <> 'Cancelled'
		ORDER BY ol.id`)
	if err != nil {
		return fmt.Errorf("query sold items: %w", err)
	}
	defer rows.Close() // runs however we leave this function: early stop, error, cancellation

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var item SoldItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return fmt.Errorf("scan sold item: %w", err)
		}
		if err := yield(item); err != nil {
			if errors.Is(err, ErrStop) {
				return nil
			}
			return err
		}
	}
	return rows.Err()
}

var _ Reader = (*SQLiteReader)(nil)
