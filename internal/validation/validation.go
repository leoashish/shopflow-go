// Package validation checks an order request before we accept it, using a chain of small checks.
package validation

import (
	"context"

	"shopflow/internal/domain"
)

// Error is one problem found in a request. Match on Code (stable), never on Message (for humans).
type Error struct {
	Code    string
	Message string
}

const (
	CodeCustomerNotFound    = "CUSTOMER_NOT_FOUND"
	CodeCustomerInactive    = "CUSTOMER_INACTIVE"
	CodeEmptyOrder          = "EMPTY_ORDER"
	CodeInvalidQuantity     = "INVALID_QUANTITY"
	CodeDuplicateProduct    = "DUPLICATE_PRODUCT"
	CodeProductNotFound     = "PRODUCT_NOT_FOUND"
	CodeInsufficientStock   = "INSUFFICIENT_STOCK"
	CodeCreditLimitExceeded = "CREDIT_LIMIT_EXCEEDED"
)

// MaxQuantityPerLine is the largest quantity allowed on one order line.
const MaxQuantityPerLine = 100

// Handler validates a request.
//
// It returns TWO different kinds of "bad news", and keeping them apart matters:
//   - the []Error is business validation ("the customer is inactive"): the request is wrong, the system is fine.
//     An empty/nil slice means the request is valid.
//   - the error is infrastructure trouble ("the database is down"): we could not decide at all.
type Handler func(ctx context.Context, req domain.OrderRequest) ([]Error, error)

// Middleware wraps a Handler with one more check. It is the CHAIN OF RESPONSIBILITY pattern in Go's favourite form
// (the same shape as net/http middleware): do my check; if it fails, answer right here WITHOUT calling next; otherwise
// hand the request on by calling next.
type Middleware func(next Handler) Handler

// CustomerGetter and ProductGetter are the tiny interfaces our checks need.
//
// Notice: an interface is defined by the CONSUMER, and only with the methods the consumer really uses (Interface
// Segregation, the "I" in SOLID, comes almost for free in Go). catalog.ProductRepository has five methods; these checks
// need one. Any type with a matching GetByID method satisfies the interface -- no "implements" keyword.
type CustomerGetter interface {
	GetByID(ctx context.Context, id int64) (domain.Customer, error)
}

type ProductGetter interface {
	GetByID(ctx context.Context, id int64) (domain.Product, error)
}
