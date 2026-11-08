package algo

import (
	"cmp"
	"container/heap"
	"fmt"
	"sort"
)

// worse reports whether a ranks BELOW b: a lower score, or the same score and a LARGER item.
func worse[T cmp.Ordered](a, b Scored[T]) bool {
	if a.Score != b.Score {
		return a.Score < b.Score
	}
	return a.Item > b.Item
}

// worstFirst is a min-heap under "worse": its root is the worst of the entries kept.
type worstFirst[T cmp.Ordered] []Scored[T]

func (h worstFirst[T]) Len() int           { return len(h) }
func (h worstFirst[T]) Less(i, j int) bool { return worse(h[i], h[j]) }
func (h worstFirst[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *worstFirst[T]) Push(x any)        { *h = append(*h, x.(Scored[T])) }
func (h *worstFirst[T]) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type TopK[T cmp.Ordered] struct {
	k    int
	best worstFirst[T]
}

func NewTopK[T cmp.Ordered](k int) (*TopK[T], error) {
	if k < 0 {
		return nil, fmt.Errorf("k must not be negative, got %d", k)
	}
	return &TopK[T]{k: k}, nil
}

func (t *TopK[T]) Add(item T, score int64) {
	if t.k == 0 {
		return
	}
	entry := Scored[T]{Item: item, Score: score}
	if len(t.best) < t.k {
		heap.Push(&t.best, entry)
		return
	}
	if worse(t.best[0], entry) { // the entry beats the current worst
		t.best[0] = entry
		heap.Fix(&t.best, 0)
	}
}

func (t *TopK[T]) Results() []Scored[T] {
	out := make([]Scored[T], len(t.best))
	copy(out, t.best)                                                     // copy: leave the heap intact so Add can continue
	sort.Slice(out, func(i, j int) bool { return worse(out[j], out[i]) }) // best first
	return out
}
