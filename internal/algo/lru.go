package algo

// LRU -- STEP 10 (part 2): a fixed-capacity cache that evicts the LEAST RECENTLY USED entry.
// A classic interview question, and genuinely useful.
//
// Requirements:
//   - Get, Put and eviction are all O(1). One structure cannot do both jobs, so combine two:
//     a map[K]*list.Element for lookup, and a container/list.List ordered by recency (front = most recent).
//     The map stores the list ELEMENTS, so you can move or remove them in O(1) without searching.
//   - NewLRU: capacity < 1 -> an error.
//   - Get(key): on a hit return the value and mark the key as most recently used.
//   - Put(key, value): new key -> insert as most recent, first evicting the least recent entry if the cache is full.
//     Existing key -> replace the value, mark most recent, size unchanged.
//   - Contains(key) is a PEEK: it does not change recency.
//   - Len() = number of entries.
//   - Safe for concurrent use by many goroutines (one sync.Mutex is the right first answer: even Get changes the list,
//     so a read-write lock would not help).
type LRU[K comparable, V any] struct {
	capacity int
}

func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	return &LRU[K, V]{capacity: capacity}, nil // TODO Step 10: reject capacity < 1
}

func (c *LRU[K, V]) Get(key K) (V, bool) {
	var zero V
	return zero, false // TODO Step 10
}

func (c *LRU[K, V]) Put(key K, value V) {} // TODO Step 10

func (c *LRU[K, V]) Contains(key K) bool { return false } // TODO Step 10

func (c *LRU[K, V]) Len() int { return 0 } // TODO Step 10
