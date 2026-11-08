package algo

import (
	"container/list"
	"fmt"
	"sync"
)

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

type LRU[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	order    *list.List          // front = most recently used, back = least recently used
	items    map[K]*list.Element // key -> its element in `order`
}

func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity < 1 {
		return nil, fmt.Errorf("capacity must be at least 1, got %d", capacity)
	}
	return &LRU[K, V]{capacity: capacity, order: list.New(), items: make(map[K]*list.Element, capacity)}, nil
}

func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruEntry[K, V]).value, true
}

func (c *LRU[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		el.Value.(*lruEntry[K, V]).value = value
		c.order.MoveToFront(el)
		return
	}
	if c.order.Len() == c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*lruEntry[K, V]).key)
	}
	c.items[key] = c.order.PushFront(&lruEntry[K, V]{key: key, value: value})
}

func (c *LRU[K, V]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}

func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
