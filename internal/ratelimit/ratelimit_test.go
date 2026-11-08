package ratelimit_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shopflow/internal/ratelimit"
	"shopflow/internal/testutil"
)

// Step 11 (part 2) -- the token bucket.

func newBucket(t *testing.T, capacity int, perSecond float64) (*ratelimit.TokenBucket, *testutil.FakeClock) {
	t.Helper()
	clk := testutil.NewFakeClock()
	b, err := ratelimit.NewTokenBucket(capacity, perSecond, clk)
	if err != nil {
		t.Fatalf("NewTokenBucket(%d, %v): %v", capacity, perSecond, err)
	}
	return b, clk
}

func take(t *testing.T, b *ratelimit.TokenBucket, n int) bool {
	t.Helper()
	ok, err := b.TryTake(n)
	if err != nil {
		t.Fatalf("TryTake(%d): %v", n, err)
	}
	return ok
}

func approx(got, want float64) bool {
	d := got - want
	return d > -1e-6 && d < 1e-6
}

func TestStep11_Bucket_StartsFull_AndRunsDry(t *testing.T) {
	b, _ := newBucket(t, 5, 1)

	for i := range 5 {
		if !take(t, b, 1) {
			t.Fatalf("take #%d failed, the bucket should start full", i+1)
		}
	}
	if take(t, b, 1) {
		t.Fatal("a 6th take succeeded from a bucket of 5")
	}
}

func TestStep11_Bucket_CanSpendSeveralTokensAtOnce(t *testing.T) {
	b, _ := newBucket(t, 10, 1)

	if !take(t, b, 7) {
		t.Fatal("take(7) failed")
	}
	if take(t, b, 7) { // only 3 left
		t.Fatal("take(7) succeeded with only 3 tokens left")
	}
	if !take(t, b, 3) {
		t.Fatal("take(3) failed")
	}
}

func TestStep11_Bucket_AFailedAttemptTakesNothing(t *testing.T) {
	b, _ := newBucket(t, 5, 1)
	take(t, b, 3) // 2 left

	if take(t, b, 3) {
		t.Fatal("take(3) succeeded with 2 tokens left")
	}
	if !take(t, b, 2) { // the 2 are still there
		t.Fatal("a failed take must not consume partial tokens")
	}
}

func TestStep11_Bucket_RefillsOverTime(t *testing.T) {
	b, clk := newBucket(t, 10, 2)
	take(t, b, 10)

	clk.Advance(time.Second)

	if got := b.Available(); !approx(got, 2.0) {
		t.Fatalf("Available = %v, want 2", got)
	}
	if !take(t, b, 2) || take(t, b, 1) {
		t.Fatal("expected exactly 2 tokens to be takeable after 1s at 2 tokens/second")
	}
}

func TestStep11_Bucket_RefillsFractionsOfATokenAndNeverExceedsCapacity(t *testing.T) {
	b, clk := newBucket(t, 10, 2)
	take(t, b, 10)

	clk.Advance(250 * time.Millisecond) // half a token
	if take(t, b, 1) {
		t.Fatal("half a token must not be enough for one")
	}
	clk.Advance(250 * time.Millisecond) // a whole token
	if !take(t, b, 1) {
		t.Fatal("one whole token should be available after 500ms at 2 tokens/second")
	}

	clk.Advance(time.Hour)
	if got := b.Available(); !approx(got, 10) {
		t.Fatalf("Available after an hour = %v, want 10 (capped at capacity)", got)
	}
}

func TestStep11_Bucket_TimeUntil_ReportsHowLongToWait(t *testing.T) {
	b, clk := newBucket(t, 10, 2)
	until := func(n int) time.Duration {
		d, err := b.TimeUntil(n)
		if err != nil {
			t.Fatalf("TimeUntil(%d): %v", n, err)
		}
		return d
	}

	if got := until(5); got != 0 {
		t.Fatalf("TimeUntil(5) with a full bucket = %v, want 0", got)
	}

	take(t, b, 10)
	if got := until(1); got != 500*time.Millisecond {
		t.Fatalf("TimeUntil(1) = %v, want 500ms", got)
	}
	if got := until(3); got != 1500*time.Millisecond {
		t.Fatalf("TimeUntil(3) = %v, want 1.5s", got)
	}

	clk.Advance(500 * time.Millisecond) // 1 token now
	if got := until(2); got != 500*time.Millisecond {
		t.Fatalf("TimeUntil(2) with 1 token = %v, want 500ms", got)
	}
}

func TestStep11_Bucket_Wait_ReturnsImmediately_WhenTokensAreAvailable(t *testing.T) {
	b, clk := newBucket(t, 3, 1)
	sleeper := testutil.NewRecordingSleeper(clk)

	if err := b.Wait(context.Background(), sleeper, 1); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if len(sleeper.Delays()) != 0 {
		t.Fatalf("waited %v, want no waiting", sleeper.Delays())
	}
}

func TestStep11_Bucket_Wait_SleepsThroughTheSleeper_UntilTokensExist(t *testing.T) {
	b, clk := newBucket(t, 10, 2)
	take(t, b, 10)
	sleeper := testutil.NewRecordingSleeper(clk) // advances the fake clock by each delay, like real sleeping

	if err := b.Wait(context.Background(), sleeper, 1); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if delays := sleeper.Delays(); len(delays) != 1 || delays[0] != 500*time.Millisecond {
		t.Fatalf("delays = %v, want exactly [500ms]", delays)
	}
	if take(t, b, 1) {
		t.Fatal("the token Wait waited for should have been spent")
	}
}

func TestStep11_Bucket_Wait_ReturnsTheContextError_WhenCancelled(t *testing.T) {
	b, clk := newBucket(t, 1, 1)
	take(t, b, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := b.Wait(ctx, testutil.NewRecordingSleeper(clk), 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestStep11_Bucket_RejectsInvalidArguments(t *testing.T) {
	clk := testutil.NewFakeClock()
	_, err := ratelimit.NewTokenBucket(0, 1, clk)
	testutil.ExpectError(t, err) // capacity 0
	_, err = ratelimit.NewTokenBucket(5, 0, clk)
	testutil.ExpectError(t, err) // refill rate 0

	b, _ := ratelimit.NewTokenBucket(5, 1, clk)
	if _, err := b.TryTake(0); !errors.Is(err, ratelimit.ErrInvalidTokens) {
		t.Errorf("TryTake(0) error = %v, want ErrInvalidTokens", err)
	}
	if _, err := b.TryTake(6); !errors.Is(err, ratelimit.ErrInvalidTokens) { // could never succeed
		t.Errorf("TryTake(6) error = %v, want ErrInvalidTokens", err)
	}
	if _, err := b.TimeUntil(6); !errors.Is(err, ratelimit.ErrInvalidTokens) {
		t.Errorf("TimeUntil(6) error = %v, want ErrInvalidTokens", err)
	}
}

func TestStep11_Bucket_IsSafeForConcurrentUse_NeverHandsOutMoreTokensThanItHas(t *testing.T) {
	b, _ := newBucket(t, 100, 1) // the clock never moves: exactly 100 tokens exist
	var granted atomic.Int64

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if ok, _ := b.TryTake(1); ok {
					granted.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if granted.Load() != 100 {
		t.Fatalf("granted %d tokens, want exactly 100", granted.Load())
	}
}
