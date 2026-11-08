package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"shopflow/internal/db"
	"shopflow/internal/domain"
)

// SQLiteCustomerRepository is GIVEN as a worked example. After you finish Step 01, compare your product repository
// with this one: the shape (query with ctx -> scan -> map errors) is the same.
type SQLiteCustomerRepository struct {
	db db.DBTX
}

func NewSQLiteCustomerRepository(d db.DBTX) *SQLiteCustomerRepository {
	return &SQLiteCustomerRepository{db: d}
}

func (r *SQLiteCustomerRepository) GetByID(ctx context.Context, id int64) (domain.Customer, error) {
	var (
		c      domain.Customer
		tier   string
		active int
	)
	// QueryRowContext, never QueryRow: the Context variants are what make cancellation and timeouts work.
	// "?" placeholders keep user input out of the SQL text (no SQL injection).
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, tier, is_active, credit_limit_cents FROM customers WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Email, &tier, &active, &c.CreditLimitCents)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.Customer{}, fmt.Errorf("customer %d: %w", id, domain.ErrNotFound) // %w keeps errors.Is working
	}
	if err != nil {
		return domain.Customer{}, fmt.Errorf("get customer %d: %w", id, err)
	}

	c.Tier = domain.CustomerTier(tier)
	c.Active = active == 1
	return c, nil
}
