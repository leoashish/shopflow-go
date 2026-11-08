package orders

import (
	"context"
	"errors"
	"fmt"

	"shopflow/internal/clock"
	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/pricing"
	"shopflow/internal/validation"
)

type Deps struct {
	Validate  validation.Handler
	Customers validation.CustomerGetter
	Products  validation.ProductGetter
	Pricing   *pricing.Calculator
	Orders    Repository
	Events    Publisher
	Clock     clock.Clock
}

type Service struct {
	d Deps
}

func NewService(d Deps) *Service { return &Service{d: d} }

func (s *Service) PlaceOrder(ctx context.Context, req domain.OrderRequest) (Result, error) {
	problems, err := s.d.Validate(ctx, req)
	if err != nil {
		return Result{}, fmt.Errorf("validate order: %w", err)
	}
	if len(problems) > 0 {
		return Result{Errors: problems}, nil
	}

	customer, err := s.d.Customers.GetByID(ctx, req.CustomerID)
	if err != nil {
		return Result{}, fmt.Errorf("load customer: %w", err)
	}

	cart := pricing.Cart{Customer: customer}
	for _, line := range req.Lines {
		p, err := s.d.Products.GetByID(ctx, line.ProductID)
		if err != nil {
			return Result{}, fmt.Errorf("load product %d: %w", line.ProductID, err)
		}
		cart.Lines = append(cart.Lines, pricing.CartLine{Product: p, Quantity: line.Quantity})
	}

	breakdown := s.d.Pricing.Calculate(cart)

	newOrder := NewOrder{CustomerID: customer.ID, TotalCents: breakdown.TotalCents, CreatedAt: s.d.Clock.Now()}
	for _, l := range cart.Lines {
		newOrder.Lines = append(newOrder.Lines, NewOrderLine{ProductID: l.Product.ID, Quantity: l.Quantity, UnitPriceCents: l.Product.PriceCents})
	}

	orderID, err := s.d.Orders.Place(ctx, newOrder)
	var outOfStock *OutOfStockError
	if errors.As(err, &outOfStock) {
		return Result{Errors: []validation.Error{{
			Code:    validation.CodeInsufficientStock,
			Message: fmt.Sprintf("product %d just went out of stock", outOfStock.ProductID),
		}}}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("place order: %w", err)
	}

	order, err := s.d.Orders.GetByID(ctx, orderID)
	if err != nil {
		return Result{}, fmt.Errorf("reload order %d: %w", orderID, err)
	}

	// The order is committed; notification trouble must not turn it into a failure. (A real app would log this.)
	_ = s.d.Events.Publish(ctx, events.OrderPlaced{OrderID: order.ID, CustomerID: order.CustomerID, TotalCents: order.TotalCents})

	return Result{Order: &order}, nil
}
