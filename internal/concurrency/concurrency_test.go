package concurrency_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shopflow/internal/concurrency"
	"shopflow/internal/testutil"
)

// Step 09 (part 1) -- bounded concurrency.   Run:  go test ./internal/concurrency ./internal/sales -run Step09 -race -v

func ints(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// mapWithTimeout runs Map but fails the test (instead of hanging) if it does not finish in time.
func mapWithTimeout[T, R any](t *testing.T, ctx context.Context, items []T, max int, fn func(context.Context, T) (R, error)) ([]R, error) {
	t.Helper()
	type outcome struct {
		res []R
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := concurrency.Map(ctx, items, max, fn)
		done <- outcome{res, err}
	}()
	o := testutil.Receive(t, done, testutil.Timeout)
	return o.res, o.err
}

// waitAll waits for wg, but gives up after d (so a mistake becomes a failed test, not a hang).
func waitAll(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestStep09_Map_ReturnsResultsInInputOrder_EvenWhenCallsFinishOutOfOrder(t *testing.T) {
	items := ints(10)

	got, err := mapWithTimeout(t, context.Background(), items, 10, func(ctx context.Context, i int) (int, error) {
		time.Sleep(time.Duration(10-i) * 5 * time.Millisecond) // item 0 finishes LAST, item 9 finishes FIRST
		return i * 2, nil
	})

	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	want := []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}
	if !slices.Equal(got, want) {
		t.Fatalf("results = %v, want %v", got, want)
	}
}

func TestStep09_Map_NeverRunsMoreThanTheLimitAtOnce(t *testing.T) {
	var running, maxSeen atomic.Int64

	got, err := mapWithTimeout(t, context.Background(), ints(20), 3, func(ctx context.Context, i int) (int, error) {
		now := running.Add(1)
		for {
			seen := maxSeen.Load()
			if now <= seen || maxSeen.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
		running.Add(-1)
		return i, nil
	})

	if err != nil || len(got) != 20 {
		t.Fatalf("Map = (%d results, %v), want 20 results", len(got), err)
	}
	if maxSeen.Load() > 3 {
		t.Fatalf("saw %d calls running at once; the limit is 3", maxSeen.Load())
	}
}

func TestStep09_Map_ReallyRunsInParallel_UpToTheLimit(t *testing.T) {
	var running atomic.Int64
	threeRunning := make(chan struct{})
	var opened atomic.Bool

	// Each call waits until THREE calls are running at the same time. A throttler that is too strict never gets there.
	got, err := mapWithTimeout(t, context.Background(), ints(6), 3, func(ctx context.Context, i int) (int, error) {
		if running.Add(1) == 3 && opened.CompareAndSwap(false, true) {
			close(threeRunning)
		}
		select {
		case <-threeRunning:
			return i, nil
		case <-time.After(2 * time.Second):
			return 0, errors.New("fewer than 3 calls ran at the same time")
		}
	})

	if err != nil || len(got) != 6 {
		t.Fatalf("Map = (%v, %v), want 6 results (is the limit being used fully?)", got, err)
	}
}

func TestStep09_Map_ReturnsTheFirstFailureThatHappened_NotAFollowUpError(t *testing.T) {
	// Item 3 fails FIRST (the root cause) -- but only once items 0-2 are running. They then notice the cancellation and
	// fail too, with errors of their own, a moment LATER. A naive "return the error with the lowest index" would
	// report item 0's follow-up.
	rootCause := errors.New("root cause")
	var othersRunning sync.WaitGroup
	othersRunning.Add(3)

	_, err := mapWithTimeout(t, context.Background(), ints(4), 4, func(ctx context.Context, i int) (int, error) {
		if i == 3 {
			if !waitAll(&othersRunning, 2*time.Second) {
				return 0, errors.New("items 0-2 never started: is the parallelism limit being honoured?")
			}
			return 0, rootCause
		}
		othersRunning.Done()
		<-ctx.Done()
		return 0, errors.New("follow-up failure caused by the cancellation")
	})

	if !errors.Is(err, rootCause) {
		t.Fatalf("error = %v, want the root cause", err)
	}
}

func TestStep09_Map_StopsStartingNewCalls_AfterAFailure(t *testing.T) {
	var started atomic.Int64

	_, err := mapWithTimeout(t, context.Background(), ints(10), 2, func(ctx context.Context, i int) (int, error) {
		started.Add(1)
		if i == 0 {
			return 0, errors.New("boom")
		}
		<-ctx.Done() // the other worker is stuck until the failure cancels it
		return 0, ctx.Err()
	})

	testutil.ExpectError(t, err)
	if got := started.Load(); got > 2 {
		t.Fatalf("%d calls were started although the very first one failed; with 2 workers at most 2 can be in flight", got)
	}
}

func TestStep09_Map_CancelsTheContextOfTheOtherCalls_OnFailure(t *testing.T) {
	sawCancellation := make(chan struct{})
	var otherRunning sync.WaitGroup
	otherRunning.Add(1)

	_, err := mapWithTimeout(t, context.Background(), []int{0, 1}, 2, func(ctx context.Context, i int) (int, error) {
		if i == 1 {
			if !waitAll(&otherRunning, 2*time.Second) { // fail only once item 0 is really running
				return 0, errors.New("item 0 never started")
			}
			return 0, errors.New("boom")
		}
		otherRunning.Done()
		<-ctx.Done()
		close(sawCancellation)
		return 0, ctx.Err()
	})

	testutil.ExpectError(t, err)
	testutil.WaitClosed(t, sawCancellation, testutil.Timeout)
}

func TestStep09_Map_DoesNotReturnWhileCallsAreStillRunning(t *testing.T) {
	var running atomic.Int64
	var othersRunning sync.WaitGroup
	othersRunning.Add(3)

	_, err := mapWithTimeout(t, context.Background(), ints(4), 4, func(ctx context.Context, i int) (int, error) {
		if i == 0 {
			if !waitAll(&othersRunning, 2*time.Second) { // fail only once the other three are really running
				return 0, errors.New("the other calls never started")
			}
			return 0, errors.New("boom")
		}
		running.Add(1)
		defer running.Add(-1)
		othersRunning.Done()
		<-ctx.Done()
		time.Sleep(40 * time.Millisecond) // slow clean-up after being cancelled
		return 0, ctx.Err()
	})

	testutil.ExpectError(t, err)
	if got := running.Load(); got != 0 {
		t.Fatalf("Map returned while %d calls were still running (goroutine leak / data race): wait for everything you started", got)
	}
}

func TestStep09_Map_ReturnsTheContextError_AndNeverCallsFn_WhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int64

	_, err := mapWithTimeout(t, ctx, ints(5), 2, func(ctx context.Context, i int) (int, error) {
		calls.Add(1)
		return i, nil
	})

	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("err=%v calls=%d, want context.Canceled and no calls", err, calls.Load())
	}
}

func TestStep09_Map_StopsWhenTheCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := mapWithTimeout(t, ctx, ints(5), 5, func(ctx context.Context, i int) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestStep09_Map_ReturnsAnEmptySlice_ForEmptyInput(t *testing.T) {
	got, err := concurrency.Map(context.Background(), []int{}, 3, func(ctx context.Context, i int) (int, error) { return i, nil })

	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Map = (%v, %v), want an empty non-nil slice and nil error", got, err)
	}
}

func TestStep09_Map_RejectsAParallelismBelowOne(t *testing.T) {
	for _, n := range []int{0, -1} {
		_, err := concurrency.Map(context.Background(), []int{1}, n, func(ctx context.Context, i int) (int, error) { return i, nil })
		if !errors.Is(err, concurrency.ErrInvalidParallelism) {
			t.Errorf("maxParallel %d: error = %v, want ErrInvalidParallelism", n, err)
		}
	}
}
