package caching

import (
	"context"
	"sync"

	"shopflow/internal/catalog"
	"shopflow/internal/domain"
)

// call is one fetch that is in flight. Everyone interested in the same id waits on done.
type call struct {
	done    chan struct{} // closed when product/err are ready
	product domain.Product
	err     error
}

type CachingProductRepository struct {
	inner catalog.ProductRepository

	mu       sync.Mutex
	cache    map[int64]domain.Product // successful results only
	inflight map[int64]*call
}

func NewCachingProductRepository(inner catalog.ProductRepository) *CachingProductRepository {
	return &CachingProductRepository{
		inner:    inner,
		cache:    map[int64]domain.Product{},
		inflight: map[int64]*call{},
	}
}

func (r *CachingProductRepository) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	r.mu.Lock()
	if p, ok := r.cache[id]; ok {
		r.mu.Unlock()
		return p, nil
	}
	c, running := r.inflight[id]
	if !running {
		c = &call{done: make(chan struct{})}
		r.inflight[id] = c
		go r.fetch(ctx, id, c) // exactly one goroutine per id does the real work
	}
	r.mu.Unlock()

	select {
	case <-c.done:
		return c.product, c.err
	case <-ctx.Done():
		return domain.Product{}, ctx.Err() // this caller gives up; the fetch continues for the others
	}
}

func (r *CachingProductRepository) fetch(ctx context.Context, id int64, c *call) {
	// WithoutCancel: the fetch is SHARED, so one impatient caller must not cancel it for everybody (keeps ctx values).
	p, err := r.inner.GetByID(context.WithoutCancel(ctx), id)

	r.mu.Lock()
	delete(r.inflight, id)
	if err == nil {
		r.cache[id] = p // only good news is cached
	}
	r.mu.Unlock()

	c.product, c.err = p, err
	close(c.done) // closing publishes the writes above to every waiter (a close happens-before a receive that sees it)
}

func (r *CachingProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	return r.inner.List(ctx)
}

func (r *CachingProductRepository) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	return r.inner.ListByCategory(ctx, category)
}

func (r *CachingProductRepository) Add(ctx context.Context, p domain.Product) (int64, error) {
	return r.inner.Add(ctx, p)
}

func (r *CachingProductRepository) UpdateStock(ctx context.Context, id int64, newStock int) error {
	err := r.inner.UpdateStock(ctx, id, newStock)

	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()

	return err
}

var _ catalog.ProductRepository = (*CachingProductRepository)(nil)
