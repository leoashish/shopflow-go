package notify_test

import (
	"context"
	"errors"
	"testing"

	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/notify"
	"shopflow/internal/testutil"
)

// Step 05 (part 2) -- Factory/registry and an observer that uses it.

var sample = events.OrderPlaced{OrderID: 77, CustomerID: 1, TotalCents: 16191}

func TestStep05_Registry_FindsChannelsByName_IgnoringCase(t *testing.T) {
	email, sms := notify.NewInMemoryChannel("email"), notify.NewInMemoryChannel("sms")
	reg, err := notify.NewRegistry(email, sms)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	for name, want := range map[string]notify.Channel{"email": email, "EMAIL": email, "Sms": sms} {
		got, err := reg.Get(name)
		if err != nil || got != want {
			t.Errorf("Get(%q) = (%v, %v), want the %s channel", name, got, err, want.Name())
		}
	}
}

func TestStep05_Registry_Get_FailsForAnUnknownChannel(t *testing.T) {
	reg, err := notify.NewRegistry(notify.NewInMemoryChannel("email"))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	if _, err := reg.Get("carrier-pigeon"); !errors.Is(err, notify.ErrUnknownChannel) {
		t.Fatalf("error = %v, want notify.ErrUnknownChannel", err)
	}
}

func TestStep05_Registry_Fails_WhenTwoChannelsShareAName(t *testing.T) {
	_, err := notify.NewRegistry(notify.NewInMemoryChannel("email"), notify.NewInMemoryChannel("EMAIL"))
	if !errors.Is(err, notify.ErrDuplicateChannel) {
		t.Fatalf("error = %v, want notify.ErrDuplicateChannel", err)
	}
}

func newNotifier(t *testing.T) (*notify.ConfirmationNotifier, *notify.InMemoryChannel, *notify.InMemoryChannel) {
	t.Helper()
	email, sms := notify.NewInMemoryChannel("email"), notify.NewInMemoryChannel("sms")
	reg, err := notify.NewRegistry(email, sms)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	customers := testutil.FakeCustomers{1: {ID: 1, Name: "Aarav", Email: "aarav@example.com", Tier: domain.TierGold, Active: true}}
	n, err := notify.NewConfirmationNotifier(customers, reg, []string{"email", "sms"})
	if err != nil {
		t.Fatalf("NewConfirmationNotifier: %v", err)
	}
	return n, email, sms
}

func TestStep05_Notifier_SendsTheSameConfirmationThroughEveryConfiguredChannel(t *testing.T) {
	n, email, sms := newNotifier(t)

	if err := n.Handle(context.Background(), sample); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	for _, ch := range []*notify.InMemoryChannel{email, sms} {
		sent := ch.Sent()
		if len(sent) != 1 {
			t.Fatalf("%s sent %d notifications, want 1", ch.Name(), len(sent))
		}
		want := notify.Notification{
			Recipient: "aarav@example.com",
			Subject:   "Order #77 confirmed",
			Body:      "Hi Aarav, your order #77 totalling $161.91 has been confirmed.",
		}
		if sent[0] != want {
			t.Fatalf("%s sent %+v, want %+v", ch.Name(), sent[0], want)
		}
	}
}

func TestStep05_Notifier_FailsFast_WhenConfiguredWithAnUnknownChannel(t *testing.T) {
	reg, _ := notify.NewRegistry(notify.NewInMemoryChannel("email"))

	_, err := notify.NewConfirmationNotifier(testutil.FakeCustomers{}, reg, []string{"email", "pigeon"})

	if !errors.Is(err, notify.ErrUnknownChannel) {
		t.Fatalf("error = %v, want notify.ErrUnknownChannel at construction time", err)
	}
}

func TestStep05_Notifier_ReturnsAnError_AndSendsNothing_WhenTheCustomerCannotBeFound(t *testing.T) {
	ch := notify.NewInMemoryChannel("email")
	reg, _ := notify.NewRegistry(ch)
	n, err := notify.NewConfirmationNotifier(testutil.FakeCustomers{}, reg, []string{"email"})
	if err != nil {
		t.Fatalf("NewConfirmationNotifier: %v", err)
	}

	err = n.Handle(context.Background(), sample)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want it to wrap domain.ErrNotFound", err)
	}
	if len(ch.Sent()) != 0 {
		t.Fatal("nothing should be sent when the customer is unknown")
	}
}

func TestStep05_Notifier_WorksAsAnObserver_WhenSubscribedToAPublisher(t *testing.T) {
	n, email, _ := newNotifier(t)
	p := events.NewPublisher()
	p.Subscribe(n) // *ConfirmationNotifier satisfies events.Handler

	if err := p.Publish(context.Background(), sample); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if len(email.Sent()) != 1 {
		t.Fatalf("email sent %d notifications, want 1", len(email.Sent()))
	}
}
