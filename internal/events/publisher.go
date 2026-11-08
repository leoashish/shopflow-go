package events

import (
	"context"

	"shopflow/internal/todo"
)

// Publisher -- STEP 05: the OBSERVER pattern's "subject". Handlers subscribe; Publish tells all of them.
//
// Requirements:
//  1. Subscribe(h) returns an unsubscribe function. Calling it removes the handler; calling it twice is harmless.
//  2. Publish runs every CURRENT subscriber CONCURRENTLY (one goroutine each, then wait for all of them with a
//     sync.WaitGroup) -- not one after another. (A test uses two handlers that each wait for the other to start:
//     run one-by-one, they deadlock.)
//  3. One failing handler never stops the others. After ALL have finished, return errors.Join(...) of every failure,
//     in SUBSCRIPTION order (collect results into a slice indexed by handler -- no shared mutable variable needed).
//     All handlers succeeded -> nil. No subscribers -> nil.
//  4. A handler that PANICS must not crash the program: recover() it in its goroutine, turn it into an error, and
//     carry on.
//  5. The context is passed to every handler.
//  6. Subscribe/unsubscribe may be called from other goroutines -- even from inside a handler, while Publish is
//     running. So take a SNAPSHOT of the subscriber list under a lock, release the lock, and only then call handlers
//     (never hold a lock while running code you don't control). Copy-on-write (build a new slice on every change) keeps
//     old snapshots valid.
type Publisher struct{}

func NewPublisher() *Publisher { return &Publisher{} }

func (p *Publisher) Subscribe(h Handler) (unsubscribe func()) {
	return func() {} // TODO Step 05
}

func (p *Publisher) Publish(ctx context.Context, e OrderPlaced) error {
	return todo.NotImplemented("Step 05: Publisher.Publish")
}
