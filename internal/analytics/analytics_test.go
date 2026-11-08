package analytics_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"shopflow/internal/analytics"
	"shopflow/internal/sales"
	"shopflow/internal/testutil"
)

// Step 10 (part 3) -- streaming aggregation + Top-K.

// fakeReader is a sales.Reader over an in-memory slice (honouring ErrStop and the context, like the real one).
type fakeReader []sales.SoldItem

func (f fakeReader) ForEach(ctx context.Context, yield func(sales.SoldItem) error) error {
	for _, it := range f {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := yield(it); err != nil {
			if errors.Is(err, sales.ErrStop) {
				return nil
			}
			return err
		}
	}
	return nil
}

func TestStep10_BestSellers_SumsUnitsPerProduct_ThenReturnsTheTopOnes(t *testing.T) {
	reader := fakeReader{{ProductID: 1, Quantity: 2}, {ProductID: 2, Quantity: 5}, {ProductID: 1, Quantity: 4}, {ProductID: 3, Quantity: 1}, {ProductID: 2, Quantity: 1}}

	got, err := analytics.BestSellers(context.Background(), reader, 2)
	if err != nil {
		t.Fatalf("BestSellers: %v", err)
	}

	// product 1: 2+4 = 6, product 2: 5+1 = 6, product 3: 1  -> a tie between 1 and 2 goes to the smaller id
	want := []analytics.ProductUnits{{ProductID: 1, Units: 6}, {ProductID: 2, Units: 6}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep10_BestSellers_IsDeterministic_DespiteRandomMapOrder(t *testing.T) {
	var reader fakeReader
	for id := int64(1); id <= 30; id++ {
		reader = append(reader, sales.SoldItem{ProductID: id, Quantity: 5}) // 30 products, all tied
	}

	first, err := analytics.BestSellers(context.Background(), reader, 5)
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		again, _ := analytics.BestSellers(context.Background(), reader, 5)
		if !slices.Equal(first, again) {
			t.Fatalf("results differ between runs: %v vs %v", first, again)
		}
	}
	if first[0].ProductID != 1 || first[4].ProductID != 5 {
		t.Fatalf("got %v, want products 1..5 (ties go to the smaller id)", first)
	}
}

func TestStep10_BestSellers_ReturnsReaderErrors_IncludingCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := analytics.BestSellers(ctx, fakeReader{{ProductID: 1, Quantity: 1}}, 3)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestStep10_BestSellers_RejectsANegativeK(t *testing.T) {
	_, err := analytics.BestSellers(context.Background(), fakeReader{}, -1)
	testutil.ExpectError(t, err)
}

func TestStep10_BestSellers_WorksOnTheRealSeededDatabase(t *testing.T) {
	// Needs Step 09's SQLiteReader.
	reader := sales.NewSQLiteReader(testutil.NewDB(t))

	top3, err := analytics.BestSellers(context.Background(), reader, 3)
	if err != nil {
		t.Fatalf("BestSellers(3): %v", err)
	}
	top4, err := analytics.BestSellers(context.Background(), reader, 4)
	if err != nil {
		t.Fatalf("BestSellers(4): %v", err)
	}

	want3 := []analytics.ProductUnits{{ProductID: 18, Units: 47}, {ProductID: 8, Units: 28}, {ProductID: 14, Units: 18}}
	if !slices.Equal(top3, want3) {
		t.Fatalf("top 3 = %v, want %v", top3, want3)
	}
	if top4[3] != (analytics.ProductUnits{ProductID: 16, Units: 18}) { // 14 and 16 both sold 18 units: lower id (14) ranks first
		t.Fatalf("4th best = %v, want product 16 with 18 units", top4[3])
	}
}
