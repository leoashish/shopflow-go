package catalog

import (
	"context"

	"shopflow/internal/db"
	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// SQLiteProductRepository -- STEP 01: implement every method.
// Read docs/steps/step-01-sqlite-repository.md first.   Run:  go test ./internal/catalog -run Step01
type SQLiteProductRepository struct {
	db db.DBTX
}

func NewSQLiteProductRepository(d db.DBTX) *SQLiteProductRepository {
	return &SQLiteProductRepository{db: d}
}

// TIP: SQLiteCustomerRepository.GetByID is a finished example of the same pattern.
// TIP: write one small scan helper and reuse it in all three read methods.

func (r *SQLiteProductRepository) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	return domain.Product{}, todo.NotImplemented("Step 01: SQLiteProductRepository.GetByID")
}

func (r *SQLiteProductRepository) List(ctx context.Context) ([]domain.Product, error) {
	return nil, todo.NotImplemented("Step 01: SQLiteProductRepository.List")
}

func (r *SQLiteProductRepository) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	return nil, todo.NotImplemented("Step 01: SQLiteProductRepository.ListByCategory")
}

func (r *SQLiteProductRepository) Add(ctx context.Context, p domain.Product) (int64, error) {
	return 0, todo.NotImplemented("Step 01: SQLiteProductRepository.Add")
}

func (r *SQLiteProductRepository) UpdateStock(ctx context.Context, id int64, newStock int) error {
	return todo.NotImplemented("Step 01: SQLiteProductRepository.UpdateStock")
}
