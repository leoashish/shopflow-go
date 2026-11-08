package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"shopflow/internal/clock"
)

// state is the STATE pattern's interface. The Breaker (the "context") delegates every decision to its CURRENT state
// object; a state changes the breaker's behaviour by asking it to switch to another state object (b.transitionTo).
//
// IMPORTANT: the Breaker calls all five methods WHILE HOLDING ITS MUTEX, so they are never called concurrently, and
// they must be quick and non-blocking: look at fields, change fields, return. Never lock the mutex yourself in a state.
type state interface {
	kind() State

	// tryEnter is called before an attempt. Return true to let the call through, false to reject it. May transition.
	tryEnter(b *Breaker) bool

	onSuccess(b *Breaker) // the attempt succeeded
	onFailure(b *Breaker) // the attempt failed (not a cancellation)
	onAborted(b *Breaker) // the caller cancelled the attempt: neither a success nor a failure
}

// Breaker -- GIVEN. The context of the State pattern. Read it carefully, then implement the three states in states.go.
//
//	Closed   --(failureThreshold consecutive failures)-->  Open
//	Open     --(openFor has elapsed, on the next call)-->  HalfOpen   (and that call becomes the trial call)
//	HalfOpen --(trial succeeds)---------------------------> Closed
//	HalfOpen --(trial fails)------------------------------> Open      (the open timer restarts)
type Breaker struct {
	mu  sync.Mutex
	cur state

	failureThreshold int
	openFor          time.Duration
	clock            clock.Clock
}

func NewBreaker(failureThreshold int, openFor time.Duration, clk clock.Clock) (*Breaker, error) {
	if failureThreshold < 1 {
		return nil, fmt.Errorf("failureThreshold must be at least 1, got %d", failureThreshold)
	}
	if openFor <= 0 {
		return nil, fmt.Errorf("openFor must be positive, got %v", openFor)
	}
	return &Breaker{cur: &closedState{}, failureThreshold: failureThreshold, openFor: openFor, clock: clk}, nil
}

// State reports the current state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cur.kind()
}

// transitionTo is called by states (with b.mu held) to switch to a new state.
func (b *Breaker) transitionTo(s state) { b.cur = s }

// Execute runs fn if the breaker lets it through, and records the outcome.
//
// The mutex is NOT held while fn runs (a slow call must not freeze every other caller).
func (b *Breaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	b.mu.Lock()
	if !b.cur.tryEnter(b) {
		b.mu.Unlock()
		return ErrOpen
	}
	b.mu.Unlock()

	// If fn panics, the call must still count as a failure -- otherwise a half-open trial slot would stay taken
	// forever. The deferred function records the failure and lets the panic carry on (no recover).
	panicked := true
	defer func() {
		if panicked {
			b.record(ctx, errors.New("panic"))
		}
	}()
	err := fn(ctx)
	panicked = false

	b.record(ctx, err)
	return err
}

func (b *Breaker) record(ctx context.Context, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case err == nil:
		b.cur.onSuccess(b)
	case ctx.Err() != nil && errors.Is(err, ctx.Err()):
		b.cur.onAborted(b) // the caller gave up: we learned nothing about the dependency's health
	default:
		b.cur.onFailure(b)
	}
}
