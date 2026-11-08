package events_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shopflow/internal/events"
	"shopflow/internal/testutil"
)

// Step 05 (part 1) -- Observer.   Run:  go test ./internal/events ./internal/notify -run Step05 -race -v

var sample = events.OrderPlaced{OrderID: 77, CustomerID: 1, TotalCents: 16191}

type counter struct{ n atomic.Int64 }

func (c *counter) Handle(ctx context.Context, e events.OrderPlaced) error {
	c.n.Add(1)
	return nil
}

func TestStep05_Publish_DeliversTheEventToEverySubscriber(t *testing.T) {
	p := events.NewPublisher()
	var mu sync.Mutex
	var got []events.OrderPlaced
	record := events.HandlerFunc(func(ctx context.Context, e events.OrderPlaced) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e)
		return nil
	})
	p.Subscribe(record)
	p.Subscribe(record)

	if err := p.Publish(context.Background(), sample); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if len(got) != 2 || got[0] != sample || got[1] != sample {
		t.Fatalf("handlers received %+v, want the sample event twice", got)
	}
}

func TestStep05_Publish_WithNoSubscribers_ReturnsNil(t *testing.T) {
	if err := events.NewPublisher().Publish(context.Background(), sample); err != nil {
		t.Fatalf("Publish with no subscribers: %v", err)
	}
}

func TestStep05_Unsubscribe_StopsDelivery_AndCallingItTwiceIsHarmless(t *testing.T) {
	p := events.NewPublisher()
	stays, leaves := &counter{}, &counter{}
	p.Subscribe(stays)
	unsubscribe := p.Subscribe(leaves)

	_ = p.Publish(context.Background(), sample)
	unsubscribe()
	unsubscribe()
	_ = p.Publish(context.Background(), sample)

	if stays.n.Load() != 2 || leaves.n.Load() != 1 {
		t.Fatalf("deliveries: stays=%d leaves=%d, want 2 and 1", stays.n.Load(), leaves.n.Load())
	}
}

func TestStep05_Publish_LetsEveryHandlerRun_AndReturnsAllTheErrors(t *testing.T) {
	p := events.NewPublisher()
	errFirst, errSecond := errors.New("first failure"), errors.New("second failure")
	healthy := &counter{}
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error { return errFirst }))
	p.Subscribe(healthy)
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error { return errSecond }))

	err := p.Publish(context.Background(), sample)

	if healthy.n.Load() != 1 {
		t.Fatal("the healthy handler did not run because others failed")
	}
	if !errors.Is(err, errFirst) || !errors.Is(err, errSecond) {
		t.Fatalf("error = %v, want BOTH failures (errors.Join)", err)
	}
}

func TestStep05_Publish_TurnsAPanickingHandlerIntoAnError_AndOthersStillRun(t *testing.T) {
	p := events.NewPublisher()
	healthy := &counter{}
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error { panic("kaboom") }))
	p.Subscribe(healthy)

	err := p.Publish(context.Background(), sample) // must not crash the test binary

	if err == nil {
		t.Fatal("Publish returned nil, want an error describing the panic")
	}
	if healthy.n.Load() != 1 {
		t.Fatal("the healthy handler did not run")
	}
}

func TestStep05_Publish_RunsHandlersConcurrently_NotOneAfterAnother(t *testing.T) {
	// Each handler announces that it started, then waits for the OTHER to have started. Run one after another,
	// the first one would wait forever.
	aStarted, bStarted := make(chan struct{}), make(chan struct{})
	waitFor := func(ch chan struct{}) error {
		select {
		case <-ch:
			return nil
		case <-time.After(2 * time.Second):
			return errors.New("the other handler never started: handlers are not running concurrently")
		}
	}
	p := events.NewPublisher()
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error { close(aStarted); return waitFor(bStarted) }))
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error { close(bStarted); return waitFor(aStarted) }))

	if err := p.Publish(context.Background(), sample); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

type ctxKey struct{}

func TestStep05_Publish_PassesTheContextToHandlers(t *testing.T) {
	var seen atomic.Value
	p := events.NewPublisher()
	p.Subscribe(events.HandlerFunc(func(ctx context.Context, e events.OrderPlaced) error {
		seen.Store(ctx.Value(ctxKey{}))
		return nil
	}))

	_ = p.Publish(context.WithValue(context.Background(), ctxKey{}, "hello"), sample)

	if seen.Load() != "hello" {
		t.Fatalf("handler saw context value %v, want %q", seen.Load(), "hello")
	}
}

func TestStep05_Publish_ToleratesAHandlerThatSubscribesDuringThePublish(t *testing.T) {
	// If Publish held its lock while running handlers, this would deadlock. A handler added mid-publish only
	// sees LATER events.
	p := events.NewPublisher()
	late := &counter{}
	var once sync.Once
	p.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error {
		once.Do(func() { p.Subscribe(late) })
		return nil
	}))

	done := make(chan error, 1)
	go func() { done <- p.Publish(context.Background(), sample) }()
	if err := testutil.Receive(t, done, testutil.Timeout); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if late.n.Load() != 0 {
		t.Fatal("a handler subscribed during Publish must not receive the event that is already being published")
	}

	_ = p.Publish(context.Background(), sample)
	if late.n.Load() != 1 {
		t.Fatalf("late handler deliveries = %d, want 1 for the next event", late.n.Load())
	}
}
