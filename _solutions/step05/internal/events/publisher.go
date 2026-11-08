package events

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

type subscription struct {
	id      int
	handler Handler
}

type Publisher struct {
	mu     sync.RWMutex
	subs   []subscription // never modified in place: every change builds a new slice (copy-on-write)
	nextID int
}

func NewPublisher() *Publisher { return &Publisher{} }

func (p *Publisher) Subscribe(h Handler) (unsubscribe func()) {
	p.mu.Lock()
	id := p.nextID
	p.nextID++
	p.subs = append(slices.Clone(p.subs), subscription{id: id, handler: h})
	p.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.subs = slices.DeleteFunc(slices.Clone(p.subs), func(s subscription) bool { return s.id == id })
		})
	}
}

func (p *Publisher) Publish(ctx context.Context, e OrderPlaced) error {
	p.mu.RLock()
	snapshot := p.subs // a stable view: later Subscribe/unsubscribe calls build NEW slices
	p.mu.RUnlock()

	if len(snapshot) == 0 {
		return nil
	}

	errs := make([]error, len(snapshot)) // one slot per handler: no two goroutines touch the same slot, so no lock needed
	var wg sync.WaitGroup
	for i, s := range snapshot {
		wg.Add(1)
		go func() { // Go 1.22: each iteration has its own i and s
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs[i] = fmt.Errorf("event handler panicked: %v", r)
				}
			}()
			errs[i] = s.handler.Handle(ctx, e)
		}()
	}
	wg.Wait()

	return errors.Join(errs...) // nil when every entry is nil
}
