package testutil

import (
	"context"
	"fmt"
	"sync"

	"shopflow/internal/domain"
)

// Hand-written fakes (no mocking library): plain structs that satisfy the small interfaces our code asks for.
// In Go an interface is satisfied implicitly, so one fake type can stand in for several consumer-defined interfaces.

// FakeCustomers is an in-memory customer store keyed by id.
type FakeCustomers map[int64]domain.Customer

func (f FakeCustomers) GetByID(ctx context.Context, id int64) (domain.Customer, error) {
	c, ok := f[id]
	if !ok {
		return domain.Customer{}, fmt.Errorf("customer %d: %w", id, domain.ErrNotFound)
	}
	return c, nil
}

// FakeProducts is an in-memory product store. It implements catalog.ProductRepository completely.
type FakeProducts struct {
	mu    sync.Mutex
	items map[int64]domain.Product
}

func NewFakeProducts(products ...domain.Product) *FakeProducts {
	f := &FakeProducts{items: map[int64]domain.Product{}}
	for _, p := range products {
		f.items[p.ID] = p
	}
	return f
}

func (f *FakeProducts) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.items[id]
	if !ok {
		return domain.Product{}, fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	return p, nil
}

func (f *FakeProducts) List(ctx context.Context) ([]domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Product, 0, len(f.items))
	for id := int64(1); len(out) < len(f.items); id++ {
		if p, ok := f.items[id]; ok {
			out = append(out, p)
		}
		if id > 10_000 {
			break
		}
	}
	return out, nil
}

func (f *FakeProducts) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	all, _ := f.List(ctx)
	var out []domain.Product
	for _, p := range all {
		if p.Category == category {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *FakeProducts) Add(ctx context.Context, p domain.Product) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := int64(len(f.items) + 1)
	p.ID = id
	f.items[id] = p
	return id, nil
}

func (f *FakeProducts) UpdateStock(ctx context.Context, id int64, newStock int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.items[id]
	if !ok {
		return fmt.Errorf("product %d: %w", id, domain.ErrNotFound)
	}
	p.Stock = newStock
	f.items[id] = p
	return nil
}

// NewProduct and NewCustomer are tiny builders so tests can state only what matters to them.
func NewProduct(id, priceCents int64, stock int) domain.Product {
	return domain.Product{ID: id, SKU: fmt.Sprintf("SKU-%d", id), Name: fmt.Sprintf("Item %d", id), PriceCents: priceCents, Stock: stock, Category: "General"}
}

func NewCustomer(id int64, tier domain.CustomerTier, active bool, creditCents int64) domain.Customer {
	return domain.Customer{ID: id, Name: fmt.Sprintf("Customer %d", id), Email: fmt.Sprintf("customer%d@example.com", id), Tier: tier, Active: active, CreditLimitCents: creditCents}
}
