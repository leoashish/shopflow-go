package algo

import "cmp"

// Scored pairs an item with its score. (Given.)
type Scored[T cmp.Ordered] struct {
	Item  T
	Score int64
}
