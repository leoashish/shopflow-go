// Package clock abstracts "what time is it?" and "wait a while", so time-dependent code can be tested without sleeping.
package clock

import (
	"context"
	"time"
)

// Clock tells the time. Code that calls time.Now() directly is hard to test; code that takes a Clock can be tested
// with a fake you control. (Dependency inversion, Go style: a one-method interface.)
type Clock interface {
	Now() time.Time
}

// System is the real clock.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Sleeper waits for a duration, but gives up early (returning ctx.Err()) if the context is cancelled.
// Retry/backoff code that really sleeps makes tests slow and flaky; with a Sleeper, tests record the requested
// delays and return instantly.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

// SystemSleeper really sleeps.
type SystemSleeper struct{}

func (SystemSleeper) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d) // not time.After: a Timer can be Stop()ped, so it does not linger when ctx wins
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
