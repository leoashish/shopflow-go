package resilience_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"shopflow/internal/resilience"
	"shopflow/internal/testutil"
)

// Step 08 -- retry with backoff and a circuit breaker (State pattern).   Run:  go test ./internal/resilience -run Step08 -race -v

const oneSecond = time.Second

var errDown = errors.New("downstream is down")

func newRetry(t *testing.T, o resilience.RetryOptions) *resilience.Retry {
	t.Helper()
	r, err := resilience.NewRetry(o)
	if err != nil {
		t.Fatalf("NewRetry: %v", err)
	}
	return r
}

func equalDurations(a, b []time.Duration) bool { return slices.Equal(a, b) }

// ======================= Retry =======================

func TestStep08_Retry_ReturnsNil_WithoutWaiting_WhenTheFirstAttemptSucceeds(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 3, BaseDelay: oneSecond, Sleeper: sleeper})

	if err := r.Do(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(sleeper.Delays()) != 0 {
		t.Fatalf("waited %v, want no waiting", sleeper.Delays())
	}
}

func TestStep08_Retry_BacksOffExponentially_UntilAnAttemptSucceeds(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 5, BaseDelay: oneSecond, Sleeper: sleeper})
	calls := 0

	err := r.Do(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return errDown
		}
		return nil
	})

	if err != nil || calls != 3 {
		t.Fatalf("Do = %v after %d calls, want success after 3", err, calls)
	}
	if want := []time.Duration{oneSecond, 2 * oneSecond}; !equalDurations(sleeper.Delays(), want) {
		t.Fatalf("delays = %v, want %v", sleeper.Delays(), want)
	}
}

func TestStep08_Retry_ReturnsTheOriginalError_WhenEveryAttemptFails_AndDoesNotWaitAfterTheLastOne(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 3, BaseDelay: oneSecond, Sleeper: sleeper})
	boom := errors.New("always fails")
	calls := 0

	err := r.Do(context.Background(), func(context.Context) error { calls++; return boom })

	if err != boom { // identity, not just errors.Is: do not wrap
		t.Fatalf("error = %v, want the very same error value back", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if want := []time.Duration{oneSecond, 2 * oneSecond}; !equalDurations(sleeper.Delays(), want) { // two waits for three attempts
		t.Fatalf("delays = %v, want %v", sleeper.Delays(), want)
	}
}

func TestStep08_Retry_GivesUpImmediately_OnErrorsTheCallerCallsPermanent(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	errBadInput := errors.New("bad input")
	r := newRetry(t, resilience.RetryOptions{
		MaxAttempts: 5, BaseDelay: oneSecond, Sleeper: sleeper,
		ShouldRetry: func(err error) bool { return !errors.Is(err, errBadInput) },
	})
	calls := 0

	err := r.Do(context.Background(), func(context.Context) error { calls++; return errBadInput })

	if !errors.Is(err, errBadInput) || calls != 1 || len(sleeper.Delays()) != 0 {
		t.Fatalf("err=%v calls=%d delays=%v, want the error after exactly 1 call and no waiting", err, calls, sleeper.Delays())
	}
}

func TestStep08_Retry_CapsTheDelayAtMaxDelay(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 5, BaseDelay: oneSecond, MaxDelay: 3 * oneSecond, Sleeper: sleeper})

	_ = r.Do(context.Background(), func(context.Context) error { return errDown })

	if want := []time.Duration{oneSecond, 2 * oneSecond, 3 * oneSecond, 3 * oneSecond}; !equalDurations(sleeper.Delays(), want) {
		t.Fatalf("delays = %v, want %v", sleeper.Delays(), want)
	}
}

func TestStep08_Retry_DoesNotOverflow_WithHugeBaseDelaysAndManyAttempts(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 50, BaseDelay: 24 * time.Hour, MaxDelay: time.Hour, Sleeper: sleeper})

	_ = r.Do(context.Background(), func(context.Context) error { return errDown })

	delays := sleeper.Delays()
	if len(delays) != 49 {
		t.Fatalf("%d delays, want 49", len(delays))
	}
	for i, d := range delays {
		if d != time.Hour {
			t.Fatalf("delay #%d = %v, want %v (the cap)", i+1, d, time.Hour)
		}
	}
}

func TestStep08_Retry_NeverRetriesCancellation(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 5, BaseDelay: oneSecond, Sleeper: sleeper})
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	err := r.Do(ctx, func(ctx context.Context) error {
		calls++
		cancel()
		return ctx.Err()
	})

	if !errors.Is(err, context.Canceled) || calls != 1 || len(sleeper.Delays()) != 0 {
		t.Fatalf("err=%v calls=%d delays=%v, want context.Canceled after exactly 1 call", err, calls, sleeper.Delays())
	}
}

func TestStep08_Retry_StopsInsteadOfWaiting_WhenCancelledBetweenAttempts(t *testing.T) {
	sleeper := testutil.NewRecordingSleeper(nil) // refuses to "sleep" on a cancelled context, like the real one
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 5, BaseDelay: oneSecond, Sleeper: sleeper})
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	err := r.Do(ctx, func(context.Context) error {
		calls++
		cancel()
		return errors.New("failed, and then the user pressed cancel")
	})

	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d, want context.Canceled after exactly 1 call", err, calls)
	}
}

func TestStep08_Retry_DoesNotStart_WhenTheContextIsAlreadyCancelled(t *testing.T) {
	r := newRetry(t, resilience.RetryOptions{MaxAttempts: 3, BaseDelay: oneSecond, Sleeper: testutil.NewRecordingSleeper(nil)})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0

	err := r.Do(ctx, func(context.Context) error { calls++; return nil })

	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("err=%v calls=%d, want context.Canceled and no calls", err, calls)
	}
}

func TestStep08_Retry_RejectsZeroAttempts(t *testing.T) {
	_, err := resilience.NewRetry(resilience.RetryOptions{MaxAttempts: 0, BaseDelay: oneSecond})
	testutil.ExpectError(t, err)
}

// ======================= Breaker =======================

func newBreaker(t *testing.T, threshold, openSeconds int) (*resilience.Breaker, *testutil.FakeClock) {
	t.Helper()
	clk := testutil.NewFakeClock()
	b, err := resilience.NewBreaker(threshold, time.Duration(openSeconds)*time.Second, clk)
	if err != nil {
		t.Fatalf("NewBreaker: %v", err)
	}
	return b, clk
}

func fail(context.Context) error    { return errDown }
func succeed(context.Context) error { return nil }

func tripOpen(t *testing.T, b *resilience.Breaker, failures int) {
	t.Helper()
	for range failures {
		if err := b.Execute(context.Background(), fail); !errors.Is(err, errDown) {
			t.Fatalf("Execute = %v, want the downstream error (the call should have been attempted)", err)
		}
	}
}

func mustState(t *testing.T, b *resilience.Breaker, want resilience.State) {
	t.Helper()
	if got := b.State(); got != want {
		t.Fatalf("breaker state = %v, want %v", got, want)
	}
}

func TestStep08_Breaker_StartsClosed_AndPassesCallsThrough(t *testing.T) {
	b, _ := newBreaker(t, 3, 10)

	mustState(t, b, resilience.Closed)
	if err := b.Execute(context.Background(), succeed); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	mustState(t, b, resilience.Closed)
}

func TestStep08_Breaker_OpensAfterTheFailureThreshold_ThenRejectsWithoutCallingTheFunction(t *testing.T) {
	b, _ := newBreaker(t, 3, 10)
	calls := 0
	counted := func(context.Context) error { calls++; return errDown }

	for range 3 {
		_ = b.Execute(context.Background(), counted)
	}

	mustState(t, b, resilience.Open)
	err := b.Execute(context.Background(), func(context.Context) error { calls++; return nil })
	if !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("Execute on an open circuit = %v, want ErrOpen", err)
	}
	if calls != 3 {
		t.Fatalf("function ran %d times, want 3: the 4th call must not reach it", calls)
	}
}

func TestStep08_Breaker_OnlyCountsConsecutiveFailures(t *testing.T) {
	b, _ := newBreaker(t, 3, 10)

	tripOpen(t, b, 2)
	if err := b.Execute(context.Background(), succeed); err != nil { // resets the streak
		t.Fatal(err)
	}
	tripOpen(t, b, 2)

	mustState(t, b, resilience.Closed)
}

func TestStep08_Breaker_StaysOpen_UntilTheOpenDurationHasPassed(t *testing.T) {
	b, clk := newBreaker(t, 1, 10)
	tripOpen(t, b, 1)

	clk.Advance(9 * time.Second)

	if err := b.Execute(context.Background(), succeed); !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("after 9s of 10s: error = %v, want ErrOpen", err)
	}
	mustState(t, b, resilience.Open)
}

func TestStep08_Breaker_AfterTheOpenDuration_LetsATrialCallThrough_AndClosesWhenItSucceeds(t *testing.T) {
	b, clk := newBreaker(t, 3, 10)
	tripOpen(t, b, 3)
	clk.Advance(10 * time.Second)

	if err := b.Execute(context.Background(), succeed); err != nil {
		t.Fatalf("trial call: %v", err)
	}

	mustState(t, b, resilience.Closed)
	tripOpen(t, b, 2) // the failure streak starts again from zero
	mustState(t, b, resilience.Closed)
}

func TestStep08_Breaker_ReopensImmediately_WhenTheTrialCallFails_AndRestartsTheTimer(t *testing.T) {
	b, clk := newBreaker(t, 1, 10)
	tripOpen(t, b, 1)
	clk.Advance(10 * time.Second)

	tripOpen(t, b, 1) // the trial call fails

	mustState(t, b, resilience.Open)
	if err := b.Execute(context.Background(), succeed); !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("right after re-opening: error = %v, want ErrOpen", err)
	}
	clk.Advance(9 * time.Second) // 9s after the RE-opening: still not enough
	if err := b.Execute(context.Background(), succeed); !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("9s after re-opening: error = %v, want ErrOpen", err)
	}
	clk.Advance(1 * time.Second) // now a full 10s
	if err := b.Execute(context.Background(), succeed); err != nil {
		t.Fatalf("after a full open period: %v", err)
	}
}

func TestStep08_Breaker_HalfOpen_AllowsOnlyOneTrialCallAtATime(t *testing.T) {
	b, clk := newBreaker(t, 1, 10)
	tripOpen(t, b, 1)
	clk.Advance(10 * time.Second)

	gate := make(chan struct{})
	trial := make(chan error, 1)
	go func() { trial <- b.Execute(context.Background(), func(context.Context) error { <-gate; return nil }) }()

	deadline := time.Now().Add(testutil.Timeout) // wait until the trial call is in flight
	for b.State() != resilience.HalfOpen {
		if time.Now().After(deadline) {
			t.Fatalf("breaker never became HalfOpen (state %v)", b.State())
		}
		time.Sleep(time.Millisecond)
	}

	if err := b.Execute(context.Background(), succeed); !errors.Is(err, resilience.ErrOpen) { // a second caller is turned away
		t.Fatalf("second caller during the trial: error = %v, want ErrOpen", err)
	}

	close(gate)
	if err := testutil.Receive(t, trial, testutil.Timeout); err != nil {
		t.Fatalf("trial call: %v", err)
	}
	mustState(t, b, resilience.Closed)
}

func TestStep08_Breaker_ACancelledTrial_FreesTheSlot_ForTheNextCaller(t *testing.T) {
	b, clk := newBreaker(t, 1, 10)
	tripOpen(t, b, 1)
	clk.Advance(10 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	trial := make(chan error, 1)
	go func() {
		trial <- b.Execute(ctx, func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() })
	}()
	testutil.WaitClosed(t, started, testutil.Timeout)
	cancel()
	if err := testutil.Receive(t, trial, testutil.Timeout); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled trial returned %v, want context.Canceled", err)
	}

	mustState(t, b, resilience.HalfOpen) // cancellation is no evidence that the service is healthy or sick
	if err := b.Execute(context.Background(), succeed); err != nil {
		t.Fatalf("a new trial should be allowed after the cancelled one: %v", err)
	}
	mustState(t, b, resilience.Closed)
}

func TestStep08_Breaker_DoesNotCountCancellationAsAFailure(t *testing.T) {
	b, _ := newBreaker(t, 1, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.Execute(ctx, func(ctx context.Context) error { return ctx.Err() })

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	mustState(t, b, resilience.Closed)
}

// ======================= Retry + Breaker together =======================

func TestStep08_RetryAndBreaker_Compose_AndRetryStopsWhenTheCircuitOpens(t *testing.T) {
	b, _ := newBreaker(t, 2, 10)
	sleeper := testutil.NewRecordingSleeper(nil)
	r := newRetry(t, resilience.RetryOptions{
		MaxAttempts: 5, BaseDelay: oneSecond, Sleeper: sleeper,
		ShouldRetry: func(err error) bool { return !errors.Is(err, resilience.ErrOpen) },
	})
	realCalls := 0

	// retry( breaker( flaky call ) ): attempts 1 and 2 fail and trip the breaker; attempt 3 is rejected with ErrOpen,
	// which we told the retry policy is not worth retrying.
	err := r.Do(context.Background(), func(ctx context.Context) error {
		return b.Execute(ctx, func(context.Context) error { realCalls++; return errDown })
	})

	if !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("error = %v, want ErrOpen", err)
	}
	if realCalls != 2 {
		t.Fatalf("the protected call ran %d times, want 2", realCalls)
	}
	mustState(t, b, resilience.Open)
	if want := []time.Duration{oneSecond, 2 * oneSecond}; !equalDurations(sleeper.Delays(), want) {
		t.Fatalf("delays = %v, want %v", sleeper.Delays(), want)
	}
}
