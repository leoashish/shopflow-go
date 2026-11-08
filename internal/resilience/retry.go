package resilience

import (
	"context"
	"time"

	"shopflow/internal/clock"
	"shopflow/internal/todo"
)

// RetryOptions configures Retry. Zero values mean "use the default".
type RetryOptions struct {
	MaxAttempts int              // TOTAL tries (1 = never retry). Must be >= 1.
	BaseDelay   time.Duration    // wait after the 1st failure; doubles after each further failure
	MaxDelay    time.Duration    // upper limit for a single wait (default 30s)
	ShouldRetry func(error) bool // return false for permanent errors (default: retry every error)
	Sleeper     clock.Sleeper    // how to wait (default clock.SystemSleeper{}); tests inject a recording one
}

// Retry -- STEP 08 (part 1): retry with exponential backoff.
//
//   - NewRetry: MaxAttempts < 1 -> an error. Fill in the defaults for MaxDelay, ShouldRetry and Sleeper.
//   - Do(ctx, fn) calls fn(ctx) up to MaxAttempts times.
//   - After failed attempt number n (1-based), if attempts remain and ShouldRetry(err) is true, wait
//     BaseDelay * 2^(n-1), but never longer than MaxDelay (so 1s, 2s, 4s, 8s ... with BaseDelay = 1s), using the
//     injected Sleeper -- NOT time.Sleep: we want testable code. Sleep(ctx, d) returns ctx.Err() if ctx ends; return that.
//   - No wait after the LAST attempt: return the final error AS IT IS (the same error value, not a wrapped copy).
//   - ShouldRetry(err) == false -> return that error immediately, no wait.
//   - Check ctx.Err() before every attempt: a cancelled context means "stop", never "retry".
//
// Beware of overflow: 2^50 * 24h does not fit in a time.Duration (an int64). Do the multiplication in float64
// (math.Pow) or clamp the exponent, and cap with MaxDelay before converting back.
type Retry struct{}

func NewRetry(o RetryOptions) (*Retry, error) {
	return nil, todo.NotImplemented("Step 08: NewRetry")
}

func (r *Retry) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return todo.NotImplemented("Step 08: Retry.Do")
}
