// Package orders places orders: validate, price, save atomically, announce.
package orders

import (
	"context"
	"fmt"
	"time"

	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/validation"
)

// NewOrderLine is a line that is ready to be saved: UnitPriceCents is a SNAPSHOT of the price at order time.
type NewOrderLine struct {
	ProductID      int64
	Quantity       int
	UnitPriceCents int64
}

type NewOrder struct {
	CustomerID int64
	Lines      []NewOrderLine
	TotalCents int64
	CreatedAt  time.Time
}

// OutOfStockError is returned by Repository.Place when a line cannot be reserved because the stock is gone (or the
// product does not exist). Detect it with errors.As:
//
//	var oos *orders.OutOfStockError
//	if errors.As(err, &oos) { ... oos.ProductID ... }
type OutOfStockError struct {
	ProductID int64
}

func (e *OutOfStockError) Error() string {
	return fmt.Sprintf("product %d does not have enough stock", e.ProductID)
}

type Repository interface {
	// Place atomically inserts the order (status 'Confirmed') and its lines AND takes the quantities out of stock.
	// Either ALL of it happens or NONE of it. Returns the new order id.
	// If any line cannot be reserved it rolls everything back and returns an *OutOfStockError.
	Place(ctx context.Context, o NewOrder) (int64, error)

	// GetByID returns domain.ErrNotFound (possibly wrapped) when there is no such order.
	GetByID(ctx context.Context, id int64) (domain.Order, error)
}

// Publisher is the one thing the service needs from the event system (a tiny, consumer-defined interface).
// *events.Publisher satisfies it.
type Publisher interface {
	Publish(ctx context.Context, e events.OrderPlaced) error
}

// Result is the outcome of PlaceOrder: either the saved Order, or the business Errors explaining why not.
// (Infrastructure failures are returned as PlaceOrder's separate `error`, just like validation.Handler.)
type Result struct {
	Order  *domain.Order
	Errors []validation.Error
}

func (r Result) Succeeded() bool { return r.Order != nil }
