package jobs

import (
	"context"

	"shopflow/internal/todo"
)

// Worker -- STEP 07: a pool of goroutines that pull jobs from a Queue and run them with a Handler.
//
//   - NewWorker(queue, handler, workers): workers < 1 -> an error.
//   - Run(ctx) starts exactly `workers` goroutines, waits for them (sync.WaitGroup) and returns:
//     nil        when the queue was closed and every job has been handled,
//     ctx.Err()  when it was stopped by cancelling ctx (graceful shutdown).
//   - Every job runs exactly once, and `workers` jobs can run at the same time.
//   - Handler returns an error  -> Failed()++ and carry on with the next job (one poison job must not stop the pool).
//   - Handler PANICS            -> recover() it, count it as Failed()++, carry on. (A panic in a goroutine you did not
//     recover would crash the whole program.)
//   - Handler succeeds          -> Processed()++.
//   - Shutdown: ctx is passed to handlers, so cancelling it tells running jobs to stop. A job that returns an error
//     BECAUSE of the shutdown (ctx.Err() != nil and errors.Is(err, ctx.Err())) counts as NEITHER processed nor failed.
//   - Counters are read from other goroutines while workers run: use sync/atomic (atomic.Int64).
type Worker struct{}

func NewWorker(q *Queue, h Handler, workers int) (*Worker, error) {
	return nil, todo.NotImplemented("Step 07: NewWorker")
}

func (w *Worker) Run(ctx context.Context) error {
	return todo.NotImplemented("Step 07: Worker.Run")
}

func (w *Worker) Processed() int { return 0 } // TODO Step 07

func (w *Worker) Failed() int { return 0 } // TODO Step 07
