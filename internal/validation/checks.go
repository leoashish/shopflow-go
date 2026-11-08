package validation

import (
	"context"

	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// notImplemented is a middleware that fails every request until you write the real thing.
func notImplemented(what string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
			return nil, todo.NotImplemented(what)
		}
	}
}

// CustomerEligibility -- STEP 03.
//   - customer missing (errors.Is(err, domain.ErrNotFound)) -> one CodeCustomerNotFound error
//   - customer inactive                                      -> one CodeCustomerInactive error
//   - any OTHER error from the getter is infrastructure trouble: return (nil, err) -- do NOT turn it into a validation error
//   - otherwise call next.
func CustomerEligibility(customers CustomerGetter) Middleware {
	return notImplemented("Step 03: CustomerEligibility")
}

// OrderLines -- STEP 03. Structural checks that need no I/O. Collect ALL problems (do not stop at the first):
//   - no lines at all                                   -> one CodeEmptyOrder error
//   - a line with Quantity < 1 or > MaxQuantityPerLine  -> one CodeInvalidQuantity error PER bad line
//   - the same ProductID on more than one line          -> one CodeDuplicateProduct error PER duplicated product
//
// If there are any errors return them (don't call next); otherwise call next.
func OrderLines() Middleware {
	return notImplemented("Step 03: OrderLines")
}

// StockAvailability -- STEP 03. For every requested line, load the product:
//   - not found (domain.ErrNotFound)    -> CodeProductNotFound
//   - Stock < requested quantity        -> CodeInsufficientStock   (exactly enough stock is fine)
//   - any other error                   -> return (nil, err) at once
//
// Report ALL failing lines (one error each). If there are none, call next.
func StockAvailability(products ProductGetter) Middleware {
	return notImplemented("Step 03: StockAvailability")
}

// CreditLimit -- STEP 03. The total of (current product price x quantity) over all lines, WITHOUT discounts, must be
// <= the customer's credit limit, otherwise one CodeCreditLimitExceeded error. Exactly equal is fine.
// If the customer or a line's product does not exist, skip it -- earlier checks report those problems.
// Other errors from the getters -> (nil, err).
func CreditLimit(customers CustomerGetter, products ProductGetter) Middleware {
	return notImplemented("Step 03: CreditLimit")
}
