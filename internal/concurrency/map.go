package concurrency

import (
	"context"

	"shopflow/internal/todo"
)

// Map -- STEP 09: run fn over every item, but only maxParallel at a time, and return the results IN INPUT ORDER.
// (Think: calling a rate-limited API for 500 products. One-by-one is slow; all 500 at once gets you banned.)
// This is a hand-made, generic cousin of errgroup from golang.org/x/sync.
//
// Requirements:
//  1. results[i] belongs to items[i], whatever order the calls finish in.
//  2. At most maxParallel calls to fn are running at once -- but that much parallelism is really used.
//  3. maxParallel < 1 -> ErrInvalidParallelism.
//  4. FAIL FAST: when fn returns an error, cancel the context passed to the other calls, start no further calls, and
//     return THE FIRST FAILURE THAT HAPPENED (the root cause) -- not a "follow-up" error that other calls returned
//     later because YOU cancelled them. On error the results are nil.
//  5. If the caller's ctx is cancelled, stop and return its error. An already-cancelled ctx means fn is never called.
//  6. NEVER return while a call to fn is still running. (Returning early leaks goroutines and lets them write to
//     memory you have already handed back -- a data race. Wait for everything you started: sync.WaitGroup.)
//  7. Empty input -> an empty (non-nil) slice and nil error.
//
// Toolbox: a derived context (context.WithCancel), a fixed pool of `workers` goroutines reading item indexes from a
// channel, sync.WaitGroup, and sync.Once to record the FIRST error (its func also cancels -- do both inside the Once
// so no follow-up error can ever win).
//
// Gotcha: when several calls fail, "the error you happen to look at first" (e.g. the lowest index) is NOT the first
// one that happened. Record the first one as it happens.
func Map[T, R any](ctx context.Context, items []T, maxParallel int, fn func(ctx context.Context, item T) (R, error)) ([]R, error) {
	return nil, todo.NotImplemented("Step 09: concurrency.Map")
}
