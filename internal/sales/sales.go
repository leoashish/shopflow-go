// Package sales reads what was sold, without loading everything into memory.
package sales

import (
	"context"
	"errors"
)

// SoldItem is one sold order line.
type SoldItem struct {
	ProductID int64
	Quantity  int
}

// ErrStop is returned by a yield callback to say "I have seen enough". ForEach then stops and returns nil.
var ErrStop = errors.New("stop iteration")

// Reader streams sold items.
//
// Why a callback (`yield`) and not a returned slice or channel? A slice loads every row into memory. A channel needs a
// goroutine and someone must drain it, or it leaks. A callback is simple, allocation-free and leak-proof: the reader
// controls the loop and can clean up (close the rows) no matter how it ends. (Go 1.23 adds iterators, `iter.Seq`, which
// are this same idea with nicer syntax -- see the PRD.)
type Reader interface {
	// ForEach calls yield for every item, in order. If yield returns ErrStop (or an error wrapping it), ForEach
	// returns nil. If yield returns any other error, ForEach stops and returns that error.
	ForEach(ctx context.Context, yield func(SoldItem) error) error
}
