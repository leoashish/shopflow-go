// Package resilience protects calls to unreliable dependencies: retry with backoff, and a circuit breaker.
package resilience

import "errors"

// State is the state of a circuit breaker.
type State int

const (
	Closed   State = iota // normal operation: calls go through
	Open                  // too many failures: calls are rejected without being attempted
	HalfOpen              // probing: exactly ONE trial call is allowed to find out whether the dependency has recovered
)

func (s State) String() string {
	switch s {
	case Closed:
		return "Closed"
	case Open:
		return "Open"
	case HalfOpen:
		return "HalfOpen"
	}
	return "Unknown"
}

// ErrOpen is returned instead of calling the protected operation while the circuit is open.
var ErrOpen = errors.New("circuit breaker is open")
