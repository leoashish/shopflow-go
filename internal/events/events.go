// Package events lets one part of the program announce "something happened" without knowing who cares.
package events

import "context"

// OrderPlaced is announced after an order has been saved.
type OrderPlaced struct {
	OrderID    int64
	CustomerID int64
	TotalCents int64
}

// Handler is an OBSERVER: anything that wants to react to an order being placed implements it.
type Handler interface {
	Handle(ctx context.Context, e OrderPlaced) error
}

// HandlerFunc turns a plain function into a Handler (same trick as http.HandlerFunc). GIVEN.
type HandlerFunc func(ctx context.Context, e OrderPlaced) error

func (f HandlerFunc) Handle(ctx context.Context, e OrderPlaced) error { return f(ctx, e) }
