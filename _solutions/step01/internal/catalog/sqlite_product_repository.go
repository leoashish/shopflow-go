package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"shopflow/internal/db"
	"shopflow/internal/domain"
)

// REFERENCE SOLUTION for Step 01. Try it yourself first!
type SQLiteProductRepository struct {
	db db.DBTX
}

func NewSQLiteProductRepository(d db.DBTX) *SQLiteProductRepository {
	return &SQLiteProductRepository{db: d}
}

const productColumns = "id, sku, name, price_cents, stock, category"

// scanner is satisfied by both *sql.Row and *sql.Rows, so one function can map either.
type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(s scanner) (domain.Product, error) {
	var p domain.Product
	err := s.Scan(&p.ID, &p.SKU, &p.Name, &p.PriceCents, &p.Stock, &p.Category)
	return p, err
}

func (r *SQLiteProductRepository) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+productColumns+" FROM products WHERE id = ?", id)
	p, err := scanProduct(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Product{}, fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("get product %d: %w", id, err)
	}
	return p, nil
}

func (r *SQLiteProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	return r.query(ctx, "SELECT "+productColumns+" FROM products ORDER BY id")
}

func (r *SQLiteProductRepository) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	return r.query(ctx, "SELECT "+productColumns+" FROM products WHERE category = ? ORDER BY id", category)
}

func (r *SQLiteProductRepository) query(ctx context.Context, query string, args ...any) ([]domain.Product, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close() // ALWAYS: an un-closed Rows keeps its connection checked out of the pool

	products := []domain.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil { // rows.Next() returning false can mean "error", not just "done"
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return products, nil
}

func (r *SQLiteProductRepository) Add(ctx context.Context, p domain.Product) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO products (sku, name, price_cents, stock, category) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		p.SKU, p.Name, p.PriceCents, p.Stock, p.Category,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("add product %q: %w", p.SKU, err)
	}
	return id, nil
}

func (r *SQLiteProductRepository) UpdateStock(ctx context.Context, id int64, newStock int) error {
	res, err := r.db.ExecContext(ctx, "UPDATE products SET stock = ? WHERE id = ?", newStock, id)
	if err != nil {
		return fmt.Errorf("update stock of product %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update stock of product %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	return nil
}
