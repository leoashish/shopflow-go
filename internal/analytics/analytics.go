// Package analytics answers business questions about what was sold.
package analytics

import (
	"context"

	"shopflow/internal/sales"
	"shopflow/internal/todo"
)

// ProductUnits is how many units of a product were sold.
type ProductUnits struct {
	ProductID int64
	Units     int64
}

// BestSellers -- STEP 10 (part 3): the k best-selling products by units sold, best first.
//
//  1. Stream the sales with reader.ForEach(ctx, ...), adding up units per product in a map[int64]int64.
//     (Memory is proportional to the number of PRODUCTS, not the number of order lines -- the payoff of Step 09.)
//  2. Feed the (productID, totalUnits) pairs to algo.NewTopK[int64](k) and return them as ProductUnits, best first.
//     Ties on units go to the smaller product id.
//  3. k < 0 -> return an error. Errors from the reader (including cancellation) are returned as they are.
//
// On the seed data the top 3 are (18, 47), (8, 28), (14, 18). Products 14 and 16 tie at 18 units; 14 wins on id.
// (The last test uses Step 09's real SQLiteReader, so finish Step 09 first.)
func BestSellers(ctx context.Context, reader sales.Reader, k int) ([]ProductUnits, error) {
	return nil, todo.NotImplemented("Step 10: analytics.BestSellers")
}
