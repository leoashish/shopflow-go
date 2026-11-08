package resilience

import "time"

type closedState struct {
	consecutiveFailures int
}

func (s *closedState) kind() State { return Closed }

func (s *closedState) tryEnter(b *Breaker) bool { return true }

func (s *closedState) onSuccess(b *Breaker) { s.consecutiveFailures = 0 }

func (s *closedState) onFailure(b *Breaker) {
	s.consecutiveFailures++
	if s.consecutiveFailures >= b.failureThreshold {
		b.transitionTo(&openState{openedAt: b.clock.Now()})
	}
}

func (s *closedState) onAborted(b *Breaker) {}

type openState struct {
	openedAt time.Time
}

func (s *openState) kind() State { return Open }

func (s *openState) tryEnter(b *Breaker) bool {
	if b.clock.Now().Sub(s.openedAt) < b.openFor {
		return false
	}
	half := &halfOpenState{}
	b.transitionTo(half)
	return half.tryEnter(b) // this caller becomes the trial call
}

func (s *openState) onSuccess(b *Breaker) {}

func (s *openState) onFailure(b *Breaker) {}

func (s *openState) onAborted(b *Breaker) {}

type halfOpenState struct {
	trialInFlight bool
}

func (s *halfOpenState) kind() State { return HalfOpen }

func (s *halfOpenState) tryEnter(b *Breaker) bool {
	if s.trialInFlight {
		return false
	}
	s.trialInFlight = true
	return true
}

func (s *halfOpenState) onSuccess(b *Breaker) { b.transitionTo(&closedState{}) }

func (s *halfOpenState) onFailure(b *Breaker) { b.transitionTo(&openState{openedAt: b.clock.Now()}) }

func (s *halfOpenState) onAborted(b *Breaker) { s.trialInFlight = false }
