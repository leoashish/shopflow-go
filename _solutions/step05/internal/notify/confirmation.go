package notify

import (
	"context"
	"fmt"

	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/money"
)

type CustomerGetter interface {
	GetByID(ctx context.Context, id int64) (domain.Customer, error)
}

type ConfirmationNotifier struct {
	customers CustomerGetter
	channels  []Channel
}

func NewConfirmationNotifier(customers CustomerGetter, registry *Registry, channelNames []string) (*ConfirmationNotifier, error) {
	channels := make([]Channel, 0, len(channelNames))
	for _, name := range channelNames {
		c, err := registry.Get(name)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return &ConfirmationNotifier{customers: customers, channels: channels}, nil
}

func (n *ConfirmationNotifier) Handle(ctx context.Context, e events.OrderPlaced) error {
	customer, err := n.customers.GetByID(ctx, e.CustomerID)
	if err != nil {
		return fmt.Errorf("load customer %d for confirmation: %w", e.CustomerID, err)
	}

	msg := Notification{
		Recipient: customer.Email,
		Subject:   fmt.Sprintf("Order #%d confirmed", e.OrderID),
		Body: fmt.Sprintf("Hi %s, your order #%d totalling %s has been confirmed.",
			customer.Name, e.OrderID, money.Format(e.TotalCents)),
	}
	for _, c := range n.channels {
		if err := c.Send(ctx, msg); err != nil {
			return fmt.Errorf("send via %s: %w", c.Name(), err)
		}
	}
	return nil
}

var _ events.Handler = (*ConfirmationNotifier)(nil)
