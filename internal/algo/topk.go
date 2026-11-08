// Package algo has small, classic data structures and algorithms.
package algo

import "cmp"

// TopK -- STEP 10 (part 1): "give me the K best items" WITHOUT sorting everything.
//
// Sorting n items costs O(n log n) time and keeps all n in memory. Instead keep only the best K seen so far in a
// MIN-HEAP whose root is the WORST of those K:
//
//	Add: heap not full           -> push it
//	     better than the root    -> replace the root and restore the heap (heap.Fix)
//	     otherwise               -> ignore it
//
// That is O(n log K) time and O(K) memory, in ONE pass -- and because items arrive through Add, the data can stream
// past (millions of rows) without ever being held in memory.
//
// Requirements:
//   - NewTopK(k): k < 0 -> an error. k == 0 is allowed and keeps nothing.
//   - Results(): at most k entries, BEST FIRST: higher Score first; on equal scores the SMALLER Item first (cmp.Less).
//     Results can be called repeatedly and Add may continue afterwards (so don't destroy the heap: copy, then sort).
//   - No entries -> an empty (non-nil) slice.
//
// Implement container/heap's interface on a slice type (Len, Less, Swap, Push, Pop) -- a rite of passage in Go.
// Careful with ties: define ONE comparison "is A worse than B?" (lower score, or equal score and LARGER item) and use
// it for the heap's Less AND for the replace-the-root decision AND for the final sort. Two slightly different
// comparisons is the classic bug.
type TopK[T cmp.Ordered] struct {
	k int
}

func NewTopK[T cmp.Ordered](k int) (*TopK[T], error) {
	return &TopK[T]{k: k}, nil // TODO Step 10: reject negative k
}

func (t *TopK[T]) Add(item T, score int64) {} // TODO Step 10

func (t *TopK[T]) Results() []Scored[T] { return nil } // TODO Step 10
