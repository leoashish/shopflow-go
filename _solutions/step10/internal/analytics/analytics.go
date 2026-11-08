package analytics

import (
	"context"

	"shopflow/internal/algo"
	"shopflow/internal/sales"
)

type ProductUnits struct {
	ProductID int64
	Units     int64
}

func BestSellers(ctx context.Context, reader sales.Reader, k int) ([]ProductUnits, error) {
	top, err := algo.NewTopK[int64](k)
	if err != nil {
		return nil, err
	}

	totals := map[int64]int64{}
	err = reader.ForEach(ctx, func(item sales.SoldItem) error {
		totals[item.ProductID] += int64(item.Quantity)
		return nil
	})
	if err != nil {
		return nil, err
	}

	for productID, units := range totals { // map order is random; TopK's tie-breaking makes the result deterministic anyway
		top.Add(productID, units)
	}

	var out []ProductUnits
	for _, s := range top.Results() {
		out = append(out, ProductUnits{ProductID: s.Item, Units: s.Score})
	}
	return out, nil
}
