package caching_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shopflow/internal/caching"
	"shopflow/internal/domain"
	"shopflow/internal/testutil"
)

// Step 04 -- Decorator pattern + cache stampede.   Run:  go test ./internal/caching -run Step04 -race -v

// countingProducts wraps FakeProducts by EMBEDDING it (every method is delegated for free) and overrides what we want
// to count or control.
type countingProducts struct {
	*testutil.FakeProducts

	getByIDCalls, listCalls, listByCategoryCalls, addCalls, updateStockCalls atomic.Int64

	onGetByID func(ctx context.Context, id int64) (domain.Product, error) // optional hook: block, fail, ...
}

func newCounting(products ...domain.Product) *countingProducts {
	return &countingProducts{FakeProducts: testutil.NewFakeProducts(products...)}
}

func (c *countingProducts) GetByID(ctx context.Context, id int64) (domain.Product, error) {
	c.getByIDCalls.Add(1)
	if c.onGetByID != nil {
		return c.onGetByID(ctx, id)
	}
	return c.FakeProducts.GetByID(ctx, id)
}

func (c *countingProducts) List(ctx context.Context) ([]domain.Product, error) {
	c.listCalls.Add(1)
	return c.FakeProducts.List(ctx)
}

func (c *countingProducts) ListByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	c.listByCategoryCalls.Add(1)
	return c.FakeProducts.ListByCategory(ctx, category)
}

func (c *countingProducts) Add(ctx context.Context, p domain.Product) (int64, error) {
	c.addCalls.Add(1)
	return c.FakeProducts.Add(ctx, p)
}

func (c *countingProducts) UpdateStock(ctx context.Context, id int64, newStock int) error {
	c.updateStockCalls.Add(1)
	return c.FakeProducts.UpdateStock(ctx, id, newStock)
}

var (
	keyboard = testutil.NewProduct(1, 8999, 10)
	mouse    = testutil.NewProduct(2, 2999, 20)
)

func TestStep04_GetByID_ReturnsTheProductFromTheInnerRepository(t *testing.T) {
	repo := caching.NewCachingProductRepository(newCounting(keyboard))

	got, err := repo.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != keyboard {
		t.Fatalf("got %+v, want %+v", got, keyboard)
	}
}

func TestStep04_GetByID_HitsTheInnerRepositoryOnlyOnce_PerProduct(t *testing.T) {
	inner := newCounting(keyboard, mouse)
	repo := caching.NewCachingProductRepository(inner)
	ctx := context.Background()

	for _, id := range []int64{1, 1, 1, 2, 2} {
		if _, err := repo.GetByID(ctx, id); err != nil {
			t.Fatalf("GetByID(%d): %v", id, err)
		}
	}

	if got := inner.getByIDCalls.Load(); got != 2 {
		t.Fatalf("inner GetByID calls = %d, want 2 (one per distinct id)", got)
	}
}

func TestStep04_GetByID_DoesNotCacheNotFound(t *testing.T) {
	inner := newCounting() // empty
	repo := caching.NewCachingProductRepository(inner)

	for range 2 {
		if _, err := repo.GetByID(context.Background(), 1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("error = %v, want domain.ErrNotFound", err)
		}
	}

	if got := inner.getByIDCalls.Load(); got != 2 {
		t.Fatalf("inner GetByID calls = %d, want 2 (a miss must not be cached)", got)
	}
}

func TestStep04_GetByID_DoesNotCacheErrors(t *testing.T) {
	inner := newCounting(keyboard)
	var failing atomic.Bool
	failing.Store(true)
	dbDown := errors.New("database exploded")
	inner.onGetByID = func(ctx context.Context, id int64) (domain.Product, error) {
		if failing.Load() {
			return domain.Product{}, dbDown
		}
		return keyboard, nil
	}
	repo := caching.NewCachingProductRepository(inner)

	if _, err := repo.GetByID(context.Background(), 1); !errors.Is(err, dbDown) {
		t.Fatalf("first call error = %v, want the database error", err)
	}

	failing.Store(false)
	got, err := repo.GetByID(context.Background(), 1) // must retry, not replay the old failure
	if err != nil || got != keyboard {
		t.Fatalf("second call = (%+v, %v), want the keyboard and no error", got, err)
	}
	if calls := inner.getByIDCalls.Load(); calls != 2 {
		t.Fatalf("inner calls = %d, want 2", calls)
	}
}

func TestStep04_GetByID_ManyConcurrentCallers_ShareOneInnerCall(t *testing.T) {
	gate := make(chan struct{})
	inner := newCounting(keyboard)
	inner.onGetByID = func(ctx context.Context, id int64) (domain.Product, error) {
		<-gate // the inner repository is "slow": it only answers when the test opens the gate
		return keyboard, nil
	}
	repo := caching.NewCachingProductRepository(inner)

	const callers = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]domain.Product, callers)
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.GetByID(context.Background(), 1)
		}()
	}
	close(start)
	time.Sleep(100 * time.Millisecond) // let every caller reach the repository while the inner call is still blocked
	close(gate)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	testutil.WaitClosed(t, done, testutil.Timeout)

	if got := inner.getByIDCalls.Load(); got != 1 {
		t.Fatalf("inner GetByID calls = %d, want exactly 1 for %d concurrent callers (cache stampede!)", got, callers)
	}
	for i := range callers {
		if errs[i] != nil || results[i] != keyboard {
			t.Fatalf("caller %d got (%+v, %v), want the keyboard", i, results[i], errs[i])
		}
	}
}

func TestStep04_GetByID_ACallerThatCancels_StopsWaiting_ButOthersStillGetTheResult(t *testing.T) {
	gate := make(chan struct{})
	inner := newCounting(keyboard)
	inner.onGetByID = func(ctx context.Context, id int64) (domain.Product, error) {
		<-gate
		return keyboard, nil
	}
	repo := caching.NewCachingProductRepository(inner)

	type result struct {
		p   domain.Product
		err error
	}
	patient := make(chan result, 1)
	go func() {
		p, err := repo.GetByID(context.Background(), 1)
		patient <- result{p, err}
	}()
	time.Sleep(50 * time.Millisecond) // the patient caller is now waiting on the in-flight fetch

	ctx, cancel := context.WithCancel(context.Background())
	impatient := make(chan result, 1)
	go func() {
		p, err := repo.GetByID(ctx, 1)
		impatient <- result{p, err}
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	r := testutil.Receive(t, impatient, testutil.Timeout) // returns even though the fetch is still blocked
	if !errors.Is(r.err, context.Canceled) {
		t.Fatalf("impatient caller error = %v, want context.Canceled", r.err)
	}

	close(gate)
	r = testutil.Receive(t, patient, testutil.Timeout)
	if r.err != nil || r.p != keyboard {
		t.Fatalf("patient caller got (%+v, %v), want the keyboard and no error", r.p, r.err)
	}
	if calls := inner.getByIDCalls.Load(); calls != 1 {
		t.Fatalf("inner calls = %d, want 1", calls)
	}
}

func TestStep04_UpdateStock_EvictsTheProduct_SoTheNextReadSeesFreshData(t *testing.T) {
	inner := newCounting(keyboard)
	repo := caching.NewCachingProductRepository(inner)
	ctx := context.Background()

	first, err := repo.GetByID(ctx, 1)
	if err != nil || first.Stock != 10 {
		t.Fatalf("first read = (%+v, %v), want stock 10", first, err)
	}
	if err := repo.UpdateStock(ctx, 1, 3); err != nil {
		t.Fatalf("UpdateStock: %v", err)
	}

	second, err := repo.GetByID(ctx, 1)
	if err != nil || second.Stock != 3 {
		t.Fatalf("read after update = (%+v, %v), want stock 3 (stale cache?)", second, err)
	}
	if inner.getByIDCalls.Load() != 2 || inner.updateStockCalls.Load() != 1 {
		t.Fatalf("inner calls: GetByID=%d UpdateStock=%d, want 2 and 1", inner.getByIDCalls.Load(), inner.updateStockCalls.Load())
	}
}

func TestStep04_UpdateStock_ReturnsTheInnerError(t *testing.T) {
	repo := caching.NewCachingProductRepository(newCounting(keyboard))

	if err := repo.UpdateStock(context.Background(), 999, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestStep04_OtherMethods_DelegateStraightToTheInnerRepository(t *testing.T) {
	inner := newCounting(keyboard, mouse)
	repo := caching.NewCachingProductRepository(inner)
	ctx := context.Background()

	for range 2 {
		list, err := repo.List(ctx)
		if err != nil || len(list) != 2 {
			t.Fatalf("List = (%d items, %v), want 2 items", len(list), err)
		}
	}
	if _, err := repo.ListByCategory(ctx, "General"); err != nil {
		t.Fatal(err)
	}
	id, err := repo.Add(ctx, testutil.NewProduct(0, 100, 1))
	if err != nil || id != 3 {
		t.Fatalf("Add = (%d, %v), want (3, nil)", id, err)
	}

	if inner.listCalls.Load() != 2 || inner.listByCategoryCalls.Load() != 1 || inner.addCalls.Load() != 1 {
		t.Fatalf("inner calls: List=%d ListByCategory=%d Add=%d, want 2, 1, 1",
			inner.listCalls.Load(), inner.listByCategoryCalls.Load(), inner.addCalls.Load())
	}
}

func TestStep04_ConcurrentMixedUse_IsRaceFree(t *testing.T) {
	// Run with -race: unsynchronised map access or unsynchronised result hand-off shows up here.
	inner := newCounting(keyboard, mouse)
	repo := caching.NewCachingProductRepository(inner)

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				id := int64(1 + (g+i)%2)
				if i%25 == 0 {
					_ = repo.UpdateStock(context.Background(), id, i)
					continue
				}
				if _, err := repo.GetByID(context.Background(), id); err != nil {
					t.Errorf("GetByID(%d): %v", id, err)
					return
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	testutil.WaitClosed(t, done, 30*time.Second)
}
