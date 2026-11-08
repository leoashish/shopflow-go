package sales_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shopflow/internal/sales"
	"shopflow/internal/testutil"
)

// Step 09 (part 2) -- streaming rows.

func collect(t *testing.T, r sales.Reader, ctx context.Context) []sales.SoldItem {
	t.Helper()
	var items []sales.SoldItem
	err := r.ForEach(ctx, func(it sales.SoldItem) error {
		items = append(items, it)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	return items
}

func TestStep09_Sales_StreamsEveryLineOfEveryNonCancelledOrder(t *testing.T) {
	d := testutil.NewDB(t)

	items := collect(t, sales.NewSQLiteReader(d), context.Background())

	var units int
	for _, it := range items {
		units += it.Quantity
	}
	if len(items) != 64 { // 69 lines in the seed, 5 of them in cancelled orders
		t.Fatalf("got %d items, want 64", len(items))
	}
	if units != 197 {
		t.Fatalf("total units = %d, want 197", units)
	}
	if cancelled := testutil.Scalar(t, d, "SELECT COUNT(*) FROM order_lines WHERE order_id IN (7, 19)"); cancelled != 5 {
		t.Fatalf("setup: cancelled orders have %d lines, want 5", cancelled)
	}
}

func TestStep09_Sales_StreamsLinesInOrderLineIDOrder(t *testing.T) {
	d := testutil.NewDB(t)

	items := collect(t, sales.NewSQLiteReader(d), context.Background())

	want := testutil.Scalar(t, d, "SELECT product_id FROM order_lines WHERE order_id = 1 ORDER BY id LIMIT 1")
	if items[0].ProductID != want {
		t.Fatalf("first item is product %d, want %d (the first line of order 1)", items[0].ProductID, want)
	}
}

func TestStep09_Sales_StopsEarly_AndReturnsNil_WhenYieldReturnsErrStop(t *testing.T) {
	r := sales.NewSQLiteReader(testutil.NewDB(t))
	seen := 0

	err := r.ForEach(context.Background(), func(sales.SoldItem) error {
		seen++
		if seen == 3 {
			return sales.ErrStop
		}
		return nil
	})

	if err != nil || seen != 3 {
		t.Fatalf("err=%v seen=%d, want nil after exactly 3 items", err, seen)
	}
}

func TestStep09_Sales_ReturnsTheCallbacksOwnError(t *testing.T) {
	r := sales.NewSQLiteReader(testutil.NewDB(t))
	boom := errors.New("callback failed")

	err := r.ForEach(context.Background(), func(sales.SoldItem) error { return boom })

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the callback's error", err)
	}
}

func TestStep09_Sales_StopsWhenTheContextIsCancelledMidway(t *testing.T) {
	r := sales.NewSQLiteReader(testutil.NewDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	seen := 0

	err := r.ForEach(ctx, func(sales.SoldItem) error {
		seen++
		if seen == 3 {
			cancel()
		}
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if seen != 3 {
		t.Fatalf("saw %d items, want 3 (iteration must stop right after the cancellation)", seen)
	}
}

func TestStep09_Sales_ReturnsTheContextError_WhenAlreadyCancelled(t *testing.T) {
	r := sales.NewSQLiteReader(testutil.NewDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.ForEach(ctx, func(sales.SoldItem) error { return nil })

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestStep09_Sales_DoesNotLeakTheConnection_WhenStoppedEarlyOrFailing(t *testing.T) {
	// A pool of ONE connection: if ForEach forgets rows.Close() when it stops early, the connection stays checked out
	// and the next query waits forever (here: until the timeout).
	d := testutil.NewDB(t)
	d.SetMaxOpenConns(1)
	r := sales.NewSQLiteReader(d)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = r.ForEach(ctx, func(sales.SoldItem) error { return sales.ErrStop })                 // stop early
	_ = r.ForEach(ctx, func(sales.SoldItem) error { return errors.New("callback failed") }) // fail early

	items := collect(t, r, ctx) // would block (and time out) if a previous call leaked the only connection
	if len(items) != 64 {
		t.Fatalf("got %d items, want 64", len(items))
	}
}
