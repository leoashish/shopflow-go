package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shopflow/internal/jobs"
	"shopflow/internal/testutil"
)

// Step 07 -- channels, back-pressure, worker pools, graceful shutdown.   Run:  go test ./internal/jobs -run Step07 -race -v

func job(i int) jobs.Job { return jobs.Job{ID: strconv.Itoa(i), Type: "test"} }

func newQueue(t *testing.T, capacity int) *jobs.Queue {
	t.Helper()
	q, err := jobs.NewQueue(capacity)
	if err != nil {
		t.Fatalf("NewQueue(%d): %v", capacity, err)
	}
	return q
}

func newWorker(t *testing.T, q *jobs.Queue, h jobs.Handler, workers int) *jobs.Worker {
	t.Helper()
	w, err := jobs.NewWorker(q, h, workers)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	return w
}

// ======================= Queue =======================

func TestStep07_Queue_RejectsACapacityBelowOne(t *testing.T) {
	for _, capacity := range []int{0, -3} {
		_, err := jobs.NewQueue(capacity)
		testutil.ExpectError(t, err)
	}
}

func TestStep07_Queue_TryEnqueue_ReturnsFalse_WhenFull(t *testing.T) {
	q := newQueue(t, 2)

	if !q.TryEnqueue(job(1)) || !q.TryEnqueue(job(2)) {
		t.Fatal("the first two TryEnqueue calls should succeed")
	}
	if q.TryEnqueue(job(3)) {
		t.Fatal("the third TryEnqueue should fail: the queue is full")
	}
}

func TestStep07_Queue_Enqueue_WaitsForRoom_WhenFull(t *testing.T) {
	q := newQueue(t, 2)
	ctx := context.Background()
	_ = q.Enqueue(ctx, job(1))
	_ = q.Enqueue(ctx, job(2))

	done := make(chan error, 1)
	go func() { done <- q.Enqueue(ctx, job(3)) }() // the queue is full: this must NOT complete yet

	select {
	case err := <-done:
		t.Fatalf("Enqueue returned (%v) while the queue was full; it should wait (back-pressure)", err)
	case <-time.After(50 * time.Millisecond):
	}

	first, err := q.Dequeue(ctx) // taking one job makes room...
	if err != nil || first.ID != "1" {
		t.Fatalf("Dequeue = (%+v, %v), want job 1", first, err)
	}
	if err := testutil.Receive(t, done, testutil.Timeout); err != nil { // ...so the waiting producer is released
		t.Fatalf("the waiting Enqueue failed: %v", err)
	}
}

func TestStep07_Queue_DeliversJobsInOrder_ThenReportsClosedOnceDrained(t *testing.T) {
	q := newQueue(t, 10)
	for i := 1; i <= 5; i++ {
		if err := q.Enqueue(context.Background(), job(i)); err != nil {
			t.Fatal(err)
		}
	}
	q.Close()

	for i := 1; i <= 5; i++ { // jobs queued before Close are still delivered
		got, err := q.Dequeue(context.Background())
		if err != nil || got.ID != strconv.Itoa(i) {
			t.Fatalf("Dequeue #%d = (%+v, %v), want job %d", i, got, err, i)
		}
	}
	if _, err := q.Dequeue(context.Background()); !errors.Is(err, jobs.ErrQueueClosed) {
		t.Fatalf("Dequeue on a drained, closed queue: error = %v, want ErrQueueClosed", err)
	}
}

func TestStep07_Queue_RejectsNewJobs_AfterClose(t *testing.T) {
	q := newQueue(t, 10)
	q.Close()

	if q.TryEnqueue(job(1)) {
		t.Fatal("TryEnqueue succeeded on a closed queue")
	}
	if err := q.Enqueue(context.Background(), job(2)); !errors.Is(err, jobs.ErrQueueClosed) {
		t.Fatalf("Enqueue error = %v, want ErrQueueClosed", err)
	}
}

func TestStep07_Queue_Close_ReleasesABlockedEnqueue_WithoutPanicking(t *testing.T) {
	q := newQueue(t, 1)
	_ = q.Enqueue(context.Background(), job(1)) // now full

	done := make(chan error, 1)
	go func() { done <- q.Enqueue(context.Background(), job(2)) }() // blocks
	time.Sleep(50 * time.Millisecond)

	q.Close() // closing the jobs channel itself here would make the blocked sender panic

	if err := testutil.Receive(t, done, testutil.Timeout); !errors.Is(err, jobs.ErrQueueClosed) {
		t.Fatalf("blocked Enqueue returned %v after Close, want ErrQueueClosed", err)
	}
}

func TestStep07_Queue_Close_CanBeCalledTwice(t *testing.T) {
	q := newQueue(t, 1)
	q.Close()
	q.Close() // must not panic ("close of closed channel")
}

func TestStep07_Queue_Dequeue_ReturnsTheContextError_WhenCancelledWhileWaiting(t *testing.T) {
	q := newQueue(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { _, err := q.Dequeue(ctx); done <- err }()

	if err := testutil.Receive(t, done, testutil.Timeout); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Dequeue error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStep07_Queue_Enqueue_ReturnsTheContextError_WhenCancelledWhileWaitingForRoom(t *testing.T) {
	q := newQueue(t, 1)
	_ = q.Enqueue(context.Background(), job(1))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- q.Enqueue(ctx, job(2)) }()

	if err := testutil.Receive(t, done, testutil.Timeout); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Enqueue error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStep07_Queue_ManyProducersAndConsumers_EveryJobArrivesExactlyOnce(t *testing.T) {
	q := newQueue(t, 16)
	const producers, perProducer = 8, 100

	var produced sync.WaitGroup
	for p := range producers {
		produced.Add(1)
		go func() {
			defer produced.Done()
			for i := range perProducer {
				if err := q.Enqueue(context.Background(), jobs.Job{ID: fmt.Sprintf("%d-%d", p, i)}); err != nil {
					t.Errorf("Enqueue: %v", err)
					return
				}
			}
		}()
	}
	go func() { produced.Wait(); q.Close() }()

	var mu sync.Mutex
	seen := map[string]int{}
	var consumed sync.WaitGroup
	for range 4 {
		consumed.Add(1)
		go func() {
			defer consumed.Done()
			for {
				j, err := q.Dequeue(context.Background())
				if err != nil {
					return
				}
				mu.Lock()
				seen[j.ID]++
				mu.Unlock()
			}
		}()
	}
	allDone := make(chan struct{})
	go func() { consumed.Wait(); close(allDone) }()
	testutil.WaitClosed(t, allDone, 20*time.Second)

	if len(seen) != producers*perProducer {
		t.Fatalf("received %d distinct jobs, want %d", len(seen), producers*perProducer)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("job %s delivered %d times, want exactly once", id, n)
		}
	}
}

// ======================= Worker =======================

func runInBackground(w *jobs.Worker, ctx context.Context) <-chan error {
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	return done
}

func TestStep07_Worker_RejectsAWorkerCountBelowOne(t *testing.T) {
	_, err := jobs.NewWorker(newQueue(t, 1), jobs.HandlerFunc(func(context.Context, jobs.Job) error { return nil }), 0)
	testutil.ExpectError(t, err)
}

func TestStep07_Worker_ProcessesEveryJobExactlyOnce_ThenRunReturnsWhenTheQueueIsClosedAndDrained(t *testing.T) {
	q := newQueue(t, 1000)
	var mu sync.Mutex
	seen := map[string]int{}
	w := newWorker(t, q, jobs.HandlerFunc(func(ctx context.Context, j jobs.Job) error {
		mu.Lock()
		defer mu.Unlock()
		seen[j.ID]++
		return nil
	}), 3)
	for i := 1; i <= 100; i++ {
		if !q.TryEnqueue(job(i)) {
			t.Fatal("setup: queue unexpectedly full")
		}
	}
	q.Close()

	err := testutil.Receive(t, runInBackground(w, context.Background()), testutil.Timeout)

	if err != nil {
		t.Fatalf("Run returned %v, want nil after the queue was closed and drained", err)
	}
	if w.Processed() != 100 || w.Failed() != 0 {
		t.Fatalf("processed=%d failed=%d, want 100 and 0", w.Processed(), w.Failed())
	}
	if len(seen) != 100 {
		t.Fatalf("%d distinct jobs handled, want 100", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("job %s handled %d times, want once", id, n)
		}
	}
}

func TestStep07_Worker_KeepsGoing_WhenSomeJobsFail(t *testing.T) {
	q := newQueue(t, 1000)
	w := newWorker(t, q, jobs.HandlerFunc(func(ctx context.Context, j jobs.Job) error {
		if n, _ := strconv.Atoi(j.ID); n%2 == 1 {
			return errors.New("poison job")
		}
		return nil
	}), 2)
	for i := 1; i <= 100; i++ {
		q.TryEnqueue(job(i))
	}
	q.Close()

	testutil.Receive(t, runInBackground(w, context.Background()), testutil.Timeout)

	if w.Processed() != 50 || w.Failed() != 50 {
		t.Fatalf("processed=%d failed=%d, want 50 and 50", w.Processed(), w.Failed())
	}
}

func TestStep07_Worker_SurvivesAPanickingHandler_AndCountsItAsFailed(t *testing.T) {
	q := newQueue(t, 100)
	w := newWorker(t, q, jobs.HandlerFunc(func(ctx context.Context, j jobs.Job) error {
		if j.ID == "3" {
			panic("handler bug")
		}
		return nil
	}), 2)
	for i := 1; i <= 10; i++ {
		q.TryEnqueue(job(i))
	}
	q.Close()

	testutil.Receive(t, runInBackground(w, context.Background()), testutil.Timeout) // an unrecovered panic would kill the test binary

	if w.Processed() != 9 || w.Failed() != 1 {
		t.Fatalf("processed=%d failed=%d, want 9 and 1", w.Processed(), w.Failed())
	}
}

func TestStep07_Worker_RunsExactlyWorkerCountJobsAtTheSameTime(t *testing.T) {
	q := newQueue(t, 100)
	var inFlight, maxInFlight atomic.Int64
	threeRunning := make(chan struct{})
	var once sync.Once

	// Every job holds still until THREE jobs are running simultaneously. With fewer than 3 workers that never happens.
	w := newWorker(t, q, jobs.HandlerFunc(func(ctx context.Context, j jobs.Job) error {
		now := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			seen := maxInFlight.Load()
			if now <= seen || maxInFlight.CompareAndSwap(seen, now) {
				break
			}
		}
		if now == 3 {
			once.Do(func() { close(threeRunning) })
		}
		select {
		case <-threeRunning:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return errors.New("fewer than 3 jobs ran at the same time")
		}
	}), 3)
	for i := 1; i <= 9; i++ {
		q.TryEnqueue(job(i))
	}
	q.Close()

	testutil.Receive(t, runInBackground(w, context.Background()), 15*time.Second)

	if w.Failed() != 0 || w.Processed() != 9 {
		t.Fatalf("processed=%d failed=%d, want 9 and 0 (did you start exactly the requested number of workers?)", w.Processed(), w.Failed())
	}
	if got := maxInFlight.Load(); got != 3 {
		t.Fatalf("at most %d jobs ran at once, want exactly 3", got)
	}
}

func TestStep07_Worker_Shutdown_CancelsRunningJobs_AndDoesNotCountThemAsFailures(t *testing.T) {
	q := newQueue(t, 10)
	started := make(chan struct{})
	var sawCancellation atomic.Bool
	w := newWorker(t, q, jobs.HandlerFunc(func(ctx context.Context, j jobs.Job) error {
		close(started)
		<-ctx.Done()
		sawCancellation.Store(true)
		return ctx.Err()
	}), 2)
	q.TryEnqueue(job(1))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(w, ctx)
	testutil.WaitClosed(t, started, testutil.Timeout)
	cancel() // "SIGTERM"

	err := testutil.Receive(t, done, testutil.Timeout)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
	if !sawCancellation.Load() {
		t.Fatal("the running handler was never told to stop (is ctx passed to the handler?)")
	}
	if w.Processed() != 0 || w.Failed() != 0 {
		t.Fatalf("processed=%d failed=%d, want 0 and 0: a job interrupted by shutdown is neither", w.Processed(), w.Failed())
	}
}

func TestStep07_Worker_StopsPromptly_WhenIdleAndCancelled(t *testing.T) {
	q := newQueue(t, 10) // stays empty
	w := newWorker(t, q, jobs.HandlerFunc(func(context.Context, jobs.Job) error { return nil }), 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(w, ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()

	if err := testutil.Receive(t, done, testutil.Timeout); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
}
