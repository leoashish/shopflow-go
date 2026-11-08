package resilience

import (
	"context"
	"fmt"
	"math"
	"time"

	"shopflow/internal/clock"
)

type RetryOptions struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	ShouldRetry func(error) bool
	Sleeper     clock.Sleeper
}

type Retry struct {
	o RetryOptions
}

func NewRetry(o RetryOptions) (*Retry, error) {
	if o.MaxAttempts < 1 {
		return nil, fmt.Errorf("MaxAttempts must be at least 1, got %d", o.MaxAttempts)
	}
	if o.MaxDelay == 0 {
		o.MaxDelay = 30 * time.Second
	}
	if o.ShouldRetry == nil {
		o.ShouldRetry = func(error) bool { return true }
	}
	if o.Sleeper == nil {
		o.Sleeper = clock.SystemSleeper{}
	}
	return &Retry{o: o}, nil
}

func (r *Retry) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err // cancelled: stop, never retry
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		if attempt >= r.o.MaxAttempts || !r.o.ShouldRetry(err) {
			return err // out of attempts, or a permanent error: the caller gets the original error back
		}
		if err := r.o.Sleeper.Sleep(ctx, r.delayAfterFailure(attempt)); err != nil {
			return err // cancelled while waiting
		}
	}
}

// delayAfterFailure is BaseDelay * 2^(failedAttempt-1), capped at MaxDelay. Computed in float64 so a huge base delay or
// attempt number cannot overflow the int64 inside time.Duration (powers of two are exact in float64).
func (r *Retry) delayAfterFailure(failedAttempt int) time.Duration {
	d := float64(r.o.BaseDelay) * math.Pow(2, float64(min(failedAttempt-1, 62)))
	if d > float64(r.o.MaxDelay) {
		return r.o.MaxDelay
	}
	return time.Duration(d)
}
