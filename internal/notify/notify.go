// Package notify sends messages to customers through pluggable channels (email, SMS, ...).
package notify

import (
	"context"
	"sync"
)

type Notification struct {
	Recipient string
	Subject   string
	Body      string
}

// Channel is one way of reaching a customer. A real EmailChannel or SmsChannel would be a new type with these two methods.
type Channel interface {
	Name() string // short unique name such as "email" or "sms"
	Send(ctx context.Context, n Notification) error
}

// InMemoryChannel just remembers what it was asked to send -- perfect for tests and local demos. GIVEN.
type InMemoryChannel struct {
	name string

	mu   sync.Mutex
	sent []Notification
}

func NewInMemoryChannel(name string) *InMemoryChannel { return &InMemoryChannel{name: name} }

func (c *InMemoryChannel) Name() string { return c.name }

func (c *InMemoryChannel) Send(ctx context.Context, n Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, n)
	return nil
}

// Sent returns a copy of everything sent so far.
func (c *InMemoryChannel) Sent() []Notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Notification(nil), c.sent...)
}
