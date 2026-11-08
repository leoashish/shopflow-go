package notify

import (
	"context"

	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/todo"
)

// CustomerGetter is the one thing this package needs from the customer store (a tiny, consumer-defined interface).
type CustomerGetter interface {
	GetByID(ctx context.Context, id int64) (domain.Customer, error)
}

// ConfirmationNotifier -- STEP 05: an observer that sends an order confirmation. It implements events.Handler.
//
//   - NewConfirmationNotifier resolves every channel name through the registry RIGHT AWAY (fail fast: a typo in the
//     configuration should fail at startup, not silently drop confirmations at 3 a.m.). Unknown name -> that error.
//   - Handle loads the customer (any error, including not-found, is returned and NOTHING is sent), then sends ONE
//     Notification through each configured channel, in the configured order:
//     Recipient = customer email
//     Subject   = fmt.Sprintf("Order #%d confirmed", orderID)
//     Body      = fmt.Sprintf("Hi %s, your order #%d totalling %s has been confirmed.", name, orderID, money.Format(total))
//   - If a channel's Send fails, return that error (wrapped with %w).
type ConfirmationNotifier struct{}

func NewConfirmationNotifier(customers CustomerGetter, registry *Registry, channelNames []string) (*ConfirmationNotifier, error) {
	return nil, todo.NotImplemented("Step 05: NewConfirmationNotifier")
}

func (n *ConfirmationNotifier) Handle(ctx context.Context, e events.OrderPlaced) error {
	return todo.NotImplemented("Step 05: ConfirmationNotifier.Handle")
}

var _ events.Handler = (*ConfirmationNotifier)(nil)
