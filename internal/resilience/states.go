package resilience

import "time"

// STEP 08 (part 2) -- implement the three states. Each state OWNS only the data it needs:
//   closed   : the number of consecutive failures so far
//   open     : when the circuit opened
//   halfOpen : whether the single trial call is already running
// See the diagram on Breaker and docs/steps/step-08-resilience.md.

var _ = time.Second // (remove this line if you don't end up needing the time package)

// closedState -- normal operation.
//
//	tryEnter  -> always true
//	onSuccess -> reset the consecutive-failure count to 0
//	onFailure -> count++; when it reaches b.failureThreshold: b.transitionTo(&openState{openedAt: b.clock.Now()})
//	onAborted -> nothing
type closedState struct {
	// TODO: the consecutive-failure counter
}

func (s *closedState) kind() State { return Closed }

func (s *closedState) tryEnter(b *Breaker) bool { return false } // TODO Step 08

func (s *closedState) onSuccess(b *Breaker) {} // TODO Step 08

func (s *closedState) onFailure(b *Breaker) {} // TODO Step 08

func (s *closedState) onAborted(b *Breaker) {} // TODO Step 08

// openState -- rejecting everything.
//
//	tryEnter -> if b.clock.Now().Sub(s.openedAt) < b.openFor: return false.
//	            Otherwise switch to a new halfOpenState (b.transitionTo) and return ITS tryEnter(b): the caller that
//	            noticed the time is up becomes the trial call.
//	onSuccess / onFailure / onAborted -> nothing (a slow call that started before the circuit opened may report in late)
type openState struct {
	openedAt time.Time
}

func (s *openState) kind() State { return Open }

func (s *openState) tryEnter(b *Breaker) bool { return false } // TODO Step 08

func (s *openState) onSuccess(b *Breaker) {} // TODO Step 08

func (s *openState) onFailure(b *Breaker) {} // TODO Step 08

func (s *openState) onAborted(b *Breaker) {} // TODO Step 08

// halfOpenState -- probing. Exactly ONE trial call may be in flight.
//
//	tryEnter  -> true for the first caller (remember that a trial is running); false for everyone else
//	onSuccess -> b.transitionTo(&closedState{})
//	onFailure -> b.transitionTo(&openState{openedAt: b.clock.Now()})   (the timer restarts)
//	onAborted -> the trial was cancelled: free the trial slot so another caller may try (stay half-open)
type halfOpenState struct {
	// TODO: is the trial call running?
}

func (s *halfOpenState) kind() State { return HalfOpen }

func (s *halfOpenState) tryEnter(b *Breaker) bool { return false } // TODO Step 08

func (s *halfOpenState) onSuccess(b *Breaker) {} // TODO Step 08

func (s *halfOpenState) onFailure(b *Breaker) {} // TODO Step 08

func (s *halfOpenState) onAborted(b *Breaker) {} // TODO Step 08
