package ratelimit

import (
	"context"
	"time"

	"shopflow/internal/clock"
	"shopflow/internal/todo"
)

// TokenBucket -- STEP 11 (part 2): the TOKEN BUCKET rate limiter. It allows bursts but enforces an average rate.
//
// A bucket holds up to `capacity` tokens and starts FULL. It refills continuously at `refillPerSecond` tokens per
// second (fractions of a token count!). Each operation spends tokens; if there aren't enough it is refused or waits.
//
// THE TRICK: there is no timer or background goroutine. Refill LAZILY: every time anybody looks at the bucket,
// compute how much time passed since the last look (from the injected clock), add elapsed*rate tokens (never above
// capacity) and remember "now". Cheap, drift-free, and trivially testable with a fake clock.
//
//   - NewTokenBucket: capacity < 1 or refillPerSecond <= 0 -> an error.
//   - Available(): refill, then report how many tokens are in the bucket (fractional!).
//   - TryTake(n): n outside 1..capacity -> (false, ErrInvalidTokens). Refill; if enough tokens, take them and return
//     (true, nil); otherwise take NOTHING and return (false, nil).
//   - TimeUntil(n): n outside 1..capacity -> (0, ErrInvalidTokens). Refill; 0 if enough tokens now, otherwise how long
//     until there will be -- rounded UP (math.Ceil on the nanoseconds), so waiting that long is guaranteed to be enough.
//   - Wait(ctx, sleeper, n): loop { ctx done? return its error; TryTake -> done; otherwise sleeper.Sleep(ctx, TimeUntil) }.
//     Use the Sleeper, never time.Sleep.
//   - Safe for concurrent use (one mutex around refill+take).
type TokenBucket struct {
	capacity int
}

func NewTokenBucket(capacity int, refillPerSecond float64, clk clock.Clock) (*TokenBucket, error) {
	return &TokenBucket{capacity: capacity}, nil // TODO Step 11: validate and store everything
}

func (b *TokenBucket) Available() float64 { return 0 } // TODO Step 11

func (b *TokenBucket) TryTake(n int) (bool, error) {
	return false, todo.NotImplemented("Step 11: TokenBucket.TryTake")
}

func (b *TokenBucket) TimeUntil(n int) (time.Duration, error) {
	return 0, todo.NotImplemented("Step 11: TokenBucket.TimeUntil")
}

func (b *TokenBucket) Wait(ctx context.Context, sleeper clock.Sleeper, n int) error {
	return todo.NotImplemented("Step 11: TokenBucket.Wait")
}
