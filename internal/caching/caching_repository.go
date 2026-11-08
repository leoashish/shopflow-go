// Package caching adds caching to a product repository without touching the repository itself.
package caching

import (
	"context"

	"shopflow/internal/catalog"
	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// CachingProductRepository -- STEP 04: the DECORATOR pattern.
// It satisfies catalog.ProductRepository AND wraps a catalog.ProductRepository. Callers cannot tell the difference
// (Liskov Substitution), and the wrapped repository knows nothing about caching.
//
// Requirements:
//  1. GetByID caches FOUND products per id: a second call does not reach the inner repository.
//  2. A "not found" result is NOT cached (the next call asks the inner repository again).
//  3. Any other error is NOT cached either.
//  4. Cache-stampede protection: if 50 goroutines ask for the same uncached id at the same moment, the inner repository
//     is called exactly ONCE and all 50 share that one result.
//  5. A caller whose context is cancelled stops WAITING at once and gets ctx.Err() -- but the shared fetch carries on
//     for the others (so run it with context.WithoutCancel(ctx), Go 1.21+, and wait with a select on ctx.Done()).
//  6. UpdateStock calls the inner repository, then evicts that product from the cache; it returns the inner error.
//  7. List, ListByCategory and Add simply delegate.
//  8. Safe for concurrent use (the tests run under the race detector).
//
// Hint for 4: a mutex-protected map of "calls in flight", each with a `done chan struct{}` that is closed when the
// result is ready. The first caller starts the fetch; everybody else just waits on done. (This is exactly what
// golang.org/x/sync/singleflight does -- you are building a tiny version of it.)
type CachingProductRepository struct {
	inner catalog.ProductRepository
}

func NewCachingProductRepository(inner catalog.ProductRepository) *CachingProductRepository {
	return &CachingProductRepository{inner: inner}
}

func (r *CachingProductRepository) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	return domain.Product{}, todo.NotImplemented("Step 04: CachingProductRepository.GetByID")
}

func (r *CachingProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	return nil, todo.NotImplemented("Step 04: CachingProductRepository.List")
}

func (r *CachingProductRepository) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	return nil, todo.NotImplemented("Step 04: CachingProductRepository.ListByCategory")
}

func (r *CachingProductRepository) Add(ctx context.Context, p domain.Product) (int64, error) {
	return 0, todo.NotImplemented("Step 04: CachingProductRepository.Add")
}

func (r *CachingProductRepository) UpdateStock(ctx context.Context, id int64, newStock int) error {
	return todo.NotImplemented("Step 04: CachingProductRepository.UpdateStock")
}

// Compile-time proof that the decorator really is a ProductRepository (a common Go idiom).
var _ catalog.ProductRepository = (*CachingProductRepository)(nil)
