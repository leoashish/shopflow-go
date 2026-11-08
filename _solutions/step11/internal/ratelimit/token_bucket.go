package ratelimit

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"shopflow/internal/clock"
)

type TokenBucket struct {
	capacity        int
	refillPerSecond float64
	clock           clock.Clock

	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

func NewTokenBucket(capacity int, refillPerSecond float64, clk clock.Clock) (*TokenBucket, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("capacity must be at least 1, got %d", capacity)
	}
	if refillPerSecond <= 0 {
		return nil, fmt.Errorf("refillPerSecond must be positive, got %v", refillPerSecond)
	}
	return &TokenBucket{
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		clock:           clk,
		tokens:          float64(capacity), // starts full
		lastRefill:      clk.Now(),
	}, nil
}

// refill adds the tokens earned since the last look. Call it with b.mu held.
func (b *TokenBucket) refill() {
	now := b.clock.Now()
	if elapsed := now.Sub(b.lastRefill).Seconds(); elapsed > 0 {
		b.tokens = math.Min(float64(b.capacity), b.tokens+elapsed*b.refillPerSecond)
	}
	b.lastRefill = now
}

func (b *TokenBucket) validate(n int) error {
	if n < 1 || n > b.capacity {
		return fmt.Errorf("%w: asked for %d, capacity is %d", ErrInvalidTokens, n, b.capacity)
	}
	return nil
}

func (b *TokenBucket) Available() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return b.tokens
}

func (b *TokenBucket) TryTake(n int) (bool, error) {
	if err := b.validate(n); err != nil {
		return false, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	if b.tokens < float64(n) {
		return false, nil // take nothing on failure
	}
	b.tokens -= float64(n)
	return true, nil
}

func (b *TokenBucket) TimeUntil(n int) (time.Duration, error) {
	if err := b.validate(n); err != nil {
		return 0, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	if b.tokens >= float64(n) {
		return 0, nil
	}
	seconds := (float64(n) - b.tokens) / b.refillPerSecond
	return time.Duration(math.Ceil(seconds * float64(time.Second))), nil // round UP: never wake up too early
}

func (b *TokenBucket) Wait(ctx context.Context, sleeper clock.Sleeper, n int) error {
	if err := b.validate(n); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, err := b.TryTake(n)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		wait, err := b.TimeUntil(n)
		if err != nil {
			return err
		}
		if err := sleeper.Sleep(ctx, wait); err != nil {
			return err
		}
	}
}
