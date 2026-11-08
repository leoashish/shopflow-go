package jobs

import (
	"context"

	"shopflow/internal/todo"
)

// Queue -- STEP 07: a bounded, goroutine-safe FIFO queue built on a buffered channel.
//
//   - NewQueue(capacity): capacity < 1 -> an error.
//   - Enqueue(ctx, job): waits while the queue is full (this waiting is called BACK-PRESSURE: a slow consumer slows
//     the producer down instead of memory growing forever). Returns ctx.Err() if ctx ends while waiting, and
//     ErrQueueClosed if the queue is (or becomes) closed.
//   - TryEnqueue(job): never waits. true if queued; false if the queue is full or closed.
//   - Dequeue(ctx): waits for a job. Returns ctx.Err() if ctx ends first. After Close it keeps returning the jobs that
//     were already queued; once they are all gone it returns ErrQueueClosed.
//   - Close(): no more jobs can be added; jobs already queued are still delivered. Calling Close twice is harmless.
//
// !!! THE BIG GO TRAP !!!  `close(ch)` on a channel that a goroutine is still SENDING to makes that sender PANIC
// ("send on closed channel"). Here a producer may be blocked in Enqueue at the very moment somebody calls Close.
// So do NOT close the jobs channel. Instead keep a SEPARATE `done chan struct{}` that Close closes (once! use
// sync.Once). Enqueue and Dequeue then `select` on {the jobs channel, done, ctx.Done()}:
//
//	Enqueue: select { case q.jobs <- job: ...; case <-q.done: return ErrQueueClosed; case <-ctx.Done(): return ctx.Err() }
//	Dequeue: prefer a ready job; if none, wait on {jobs, done, ctx.Done()}; when done fires, drain what is left.
//
// Also check the "already closed / already cancelled" cases FIRST: when several select cases are ready Go picks one at
// random, and "Close() returned, so Enqueue must fail" should not be a coin flip.
type Queue struct{}

func NewQueue(capacity int) (*Queue, error) {
	return nil, todo.NotImplemented("Step 07: NewQueue")
}

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	return todo.NotImplemented("Step 07: Queue.Enqueue")
}

func (q *Queue) TryEnqueue(job Job) bool { return false } // TODO Step 07

func (q *Queue) Dequeue(ctx context.Context) (Job, error) {
	return Job{}, todo.NotImplemented("Step 07: Queue.Dequeue")
}

func (q *Queue) Close() {} // TODO Step 07
