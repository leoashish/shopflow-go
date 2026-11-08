package jobs

import (
	"context"
	"fmt"
	"sync"
)

type Queue struct {
	jobs      chan Job      // buffered: its capacity is the queue's capacity. NEVER closed (see the PRD).
	done      chan struct{} // closed by Close: "no more jobs will be accepted"
	closeOnce sync.Once
}

func NewQueue(capacity int) (*Queue, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("queue capacity must be at least 1, got %d", capacity)
	}
	return &Queue{jobs: make(chan Job, capacity), done: make(chan struct{})}, nil
}

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	// Cheap checks first, so the common "already closed / already cancelled" cases are deterministic.
	select {
	case <-q.done:
		return ErrQueueClosed
	default:
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case q.jobs <- job:
		return nil
	case <-q.done:
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) TryEnqueue(job Job) bool {
	select {
	case <-q.done:
		return false
	default:
	}
	select {
	case q.jobs <- job:
		return true
	default:
		return false
	}
}

func (q *Queue) Dequeue(ctx context.Context) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}

	select { // prefer a job that is already waiting
	case job := <-q.jobs:
		return job, nil
	default:
	}

	select {
	case job := <-q.jobs:
		return job, nil
	case <-q.done:
		select { // closed: hand out whatever is still queued, then report closed
		case job := <-q.jobs:
			return job, nil
		default:
			return Job{}, ErrQueueClosed
		}
	case <-ctx.Done():
		return Job{}, ctx.Err()
	}
}

func (q *Queue) Close() {
	q.closeOnce.Do(func() { close(q.done) }) // closing a channel twice panics: Once makes Close idempotent
}
