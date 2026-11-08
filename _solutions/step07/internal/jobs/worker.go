package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type Worker struct {
	queue   *Queue
	handler Handler
	workers int

	processed atomic.Int64
	failed    atomic.Int64
}

func NewWorker(q *Queue, h Handler, workers int) (*Worker, error) {
	if workers < 1 {
		return nil, fmt.Errorf("need at least one worker, got %d", workers)
	}
	return &Worker{queue: q, handler: h, workers: workers}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for range w.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.loop(ctx)
		}()
	}
	wg.Wait()

	return ctx.Err() // nil if we finished because the queue was closed and drained
}

func (w *Worker) loop(ctx context.Context) {
	for {
		job, err := w.queue.Dequeue(ctx)
		if err != nil {
			return // queue closed and drained, or shutting down
		}
		w.run(ctx, job)
	}
}

func (w *Worker) run(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			w.failed.Add(1) // a panicking job is a failed job, not a crashed program
		}
	}()

	err := w.handler.Handle(ctx, job)
	switch {
	case err == nil:
		w.processed.Add(1)
	case ctx.Err() != nil && errors.Is(err, ctx.Err()):
		// interrupted by shutdown: neither a success nor a failure
	default:
		w.failed.Add(1)
	}
}

func (w *Worker) Processed() int { return int(w.processed.Load()) }

func (w *Worker) Failed() int { return int(w.failed.Load()) }
