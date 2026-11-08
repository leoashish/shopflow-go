package orders

import (
	"context"

	"shopflow/internal/clock"
	"shopflow/internal/domain"
	"shopflow/internal/pricing"
	"shopflow/internal/todo"
	"shopflow/internal/validation"
)

// Deps is everything the Service needs, all of it ABSTRACTIONS or small collaborators (Dependency Inversion).
// The service never creates a repository or opens a database itself -- which is exactly why it is testable.
type Deps struct {
	Validate  validation.Handler
	Customers validation.CustomerGetter
	Products  validation.ProductGetter
	Pricing   *pricing.Calculator
	Orders    Repository
	Events    Publisher
	Clock     clock.Clock
}

// Service -- STEP 06: the application service that orchestrates everything from steps 1-5.
//
// PlaceOrder(ctx, req):
//  1. Run d.Validate. If it returns an error -> return (Result{}, err). If it returns validation errors -> return
//     (Result{Errors: errs}, nil) and touch nothing.
//  2. Load the customer and the products (validation proved they exist) and price the cart with d.Pricing.
//  3. Build the NewOrder: line unit price = the product's CURRENT price (an undiscounted snapshot); total =
//     the breakdown's TotalCents; CreatedAt = d.Clock.Now().
//  4. d.Orders.Place. If it fails with an *OutOfStockError (errors.As) -- someone bought the last unit between our
//     validation and our write -- return (Result{Errors: [one validation.CodeInsufficientStock error]}, nil).
//     Any other error -> return it (wrapped with %w).
//  5. Reload the saved order (d.Orders.GetByID) and publish events.OrderPlaced{OrderID, CustomerID, TotalCents}.
//     The order is ALREADY committed, so if publishing fails, IGNORE that error (a real app would log it).
//  6. Return (Result{Order: &order}, nil).
//
// Context cancellation must surface as an error from the first step that notices it.
type Service struct {
	d Deps
}

func NewService(d Deps) *Service { return &Service{d: d} }

func (s *Service) PlaceOrder(ctx context.Context, req domain.OrderRequest) (Result, error) {
	return Result{}, todo.NotImplemented("Step 06: Service.PlaceOrder")
}
