// Package jobs is a small background-work system: a bounded queue and a pool of workers.
package jobs

import (
	"context"
	"errors"
)

// Job is a unit of background work, e.g. Type "send-invoice", Payload `{"orderId":31}`.
type Job struct {
	ID      string
	Type    string
	Payload string
}

// Handler does the actual work of one job.
type Handler interface {
	Handle(ctx context.Context, job Job) error
}

// HandlerFunc turns a plain function into a Handler (like http.HandlerFunc). GIVEN.
type HandlerFunc func(ctx context.Context, job Job) error

func (f HandlerFunc) Handle(ctx context.Context, job Job) error { return f(ctx, job) }

// ErrQueueClosed is returned when the queue has been closed (and, for Dequeue, also fully drained).
var ErrQueueClosed = errors.New("queue closed")
