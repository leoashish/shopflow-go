package validation

import (
	"context"

	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// NewOrderValidator -- STEP 03. Wires the checks into one chain, in this order (cheap/fundamental checks first):
//
//	CustomerEligibility -> OrderLines -> StockAvailability -> CreditLimit
func NewOrderValidator(customers CustomerGetter, products ProductGetter) Handler {
	return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
		return nil, todo.NotImplemented("Step 03: NewOrderValidator")
	}
}
