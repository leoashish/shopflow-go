// Package catalog is the data-access layer for products and customers.
package catalog

import (
	"context"

	"shopflow/internal/domain"
)

// ProductRepository is the abstraction the rest of the app depends on (Dependency Inversion).
// Not-found is reported with an error that satisfies errors.Is(err, domain.ErrNotFound).
type ProductRepository interface {
	// GetByID returns domain.ErrNotFound (possibly wrapped) when no product has this id.
	GetByID(ctx context.Context, id int64) (domain.Product, error)

	// List returns all products ordered by id.
	List(ctx context.Context) ([]domain.Product, error)

	// ListByCategory returns the products whose category equals category (exact match), ordered by id.
	ListByCategory(ctx context.Context, category string) ([]domain.Product, error)

	// Add inserts the product (its ID is ignored) and returns the new database id.
	Add(ctx context.Context, p domain.Product) (int64, error)

	// UpdateStock sets the stock level. It returns domain.ErrNotFound (possibly wrapped) when no product has this id.
	UpdateStock(ctx context.Context, id int64, newStock int) error
}

// CustomerRepository reads customers.
type CustomerRepository interface {
	GetByID(ctx context.Context, id int64) (domain.Customer, error)
}
