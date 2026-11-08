package concurrency

import (
	"context"
	"sync"
)

func Map[T, R any](ctx context.Context, items []T, maxParallel int, fn func(ctx context.Context, item T) (R, error)) ([]R, error) {
	if maxParallel < 1 {
		return nil, ErrInvalidParallelism
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []R{}, nil
	}

	runCtx, cancel := context.WithCancel(ctx) // cancelled on the first failure, or when the caller cancels
	defer cancel()

	results := make([]R, len(items))
	var (
		once     sync.Once
		firstErr error
	)
	fail := func(err error) {
		once.Do(func() { // the FIRST failure wins; recording it and cancelling happen together, so no follow-up can overtake it
			firstErr = err
			cancel()
		})
	}

	indexes := make(chan int)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { // feeder: hands out indexes until done or cancelled
		defer wg.Done()
		defer close(indexes) // safe: this goroutine is the ONLY sender
		for i := range items {
			select {
			case indexes <- i:
			case <-runCtx.Done():
				return
			}
		}
	}()

	for range min(maxParallel, len(items)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				if runCtx.Err() != nil {
					return // somebody failed (or the caller cancelled): do not START anything new
				}
				r, err := fn(runCtx, items[i])
				if err != nil {
					fail(err)
					return
				}
				results[i] = r // each index is written by exactly one goroutine: no lock needed
			}
		}()
	}

	wg.Wait() // never return while anything we started is still running

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
