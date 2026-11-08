package catalog_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"shopflow/internal/catalog"
	"shopflow/internal/domain"
	"shopflow/internal/testutil"
)

// Step 01 -- repository over database/sql.   Run:  go test ./internal/catalog -run Step01 -v

func newRepo(t *testing.T) (*catalog.SQLiteProductRepository, *sql.DB) {
	t.Helper()
	d := testutil.NewDB(t)
	return catalog.NewSQLiteProductRepository(d), d
}

func ids(products []domain.Product) []int64 {
	out := make([]int64, 0, len(products))
	for _, p := range products {
		out = append(out, p.ID)
	}
	return out
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStep01_GetByID_ReturnsTheProduct(t *testing.T) {
	repo, _ := newRepo(t)

	got, err := repo.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetByID(1): unexpected error: %v", err)
	}
	want := domain.Product{ID: 1, SKU: "KB-MECH-01", Name: "Mechanical Keyboard", PriceCents: 8999, Stock: 50, Category: "Peripherals"}
	if got != want {
		t.Fatalf("GetByID(1) = %+v, want %+v", got, want)
	}
}

func TestStep01_GetByID_ReturnsErrNotFound_ForAnUnknownID(t *testing.T) {
	repo, _ := newRepo(t)

	_, err := repo.GetByID(context.Background(), 9999)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID(9999) error = %v, want an error satisfying errors.Is(err, domain.ErrNotFound)", err)
	}
}

func TestStep01_List_ReturnsEveryProduct_OrderedByID(t *testing.T) {
	repo, _ := newRepo(t)

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(got) != 20 {
		t.Fatalf("List returned %d products, want 20", len(got))
	}
	for i, p := range got {
		if p.ID != int64(i+1) {
			t.Fatalf("List()[%d].ID = %d, want %d (ordered by id)", i, p.ID, i+1)
		}
	}
}

func TestStep01_ListByCategory_ReturnsOnlyMatchingProducts(t *testing.T) {
	repo, _ := newRepo(t)

	got, err := repo.ListByCategory(context.Background(), "Audio")
	if err != nil {
		t.Fatalf("ListByCategory: unexpected error: %v", err)
	}
	if want := []int64{6, 7, 8, 20}; !equalInts(ids(got), want) {
		t.Fatalf("ListByCategory(Audio) ids = %v, want %v", ids(got), want)
	}
}

func TestStep01_ListByCategory_ReturnsAnEmptyList_WhenNothingMatches(t *testing.T) {
	repo, _ := newRepo(t)

	got, err := repo.ListByCategory(context.Background(), "Unicycles")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d products, want none", len(got))
	}
}

func TestStep01_ListByCategory_TreatsInputAsData_NotAsSQL(t *testing.T) {
	// If you build the SQL with string concatenation/Sprintf, this "injection" returns every row.
	repo, _ := newRepo(t)

	got, err := repo.ListByCategory(context.Background(), "Audio' OR '1'='1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SQL injection worked: got %d products, want none", len(got))
	}
}

func TestStep01_Add_InsertsTheProduct_AndReturnsTheNewID(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()

	id, err := repo.Add(ctx, domain.Product{SKU: "NEW-001", Name: "Brand New Thing", PriceCents: 1234, Stock: 7, Category: "Accessories"})
	if err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	if id != 21 { // 20 seeded products
		t.Fatalf("Add returned id %d, want 21", id)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID(%d): %v", id, err)
	}
	want := domain.Product{ID: 21, SKU: "NEW-001", Name: "Brand New Thing", PriceCents: 1234, Stock: 7, Category: "Accessories"}
	if got != want {
		t.Fatalf("saved product = %+v, want %+v", got, want)
	}
}

func TestStep01_Add_Fails_WhenTheSKUAlreadyExists(t *testing.T) {
	repo, _ := newRepo(t)

	// KB-MECH-01 is in the seed data, and sku has a UNIQUE constraint: the database must say no.
	_, err := repo.Add(context.Background(), domain.Product{SKU: "KB-MECH-01", Name: "Duplicate", PriceCents: 100, Stock: 1, Category: "Peripherals"})

	testutil.ExpectError(t, err)
	if errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a constraint violation must not be reported as ErrNotFound: %v", err)
	}
}

func TestStep01_UpdateStock_PersistsTheNewStock(t *testing.T) {
	repo, d := newRepo(t)

	if err := repo.UpdateStock(context.Background(), 1, 5); err != nil {
		t.Fatalf("UpdateStock: unexpected error: %v", err)
	}
	if got := testutil.Scalar(t, d, "SELECT stock FROM products WHERE id = 1"); got != 5 {
		t.Fatalf("stock in the database = %d, want 5", got)
	}
}

func TestStep01_UpdateStock_ReturnsErrNotFound_ForAnUnknownProduct(t *testing.T) {
	repo, _ := newRepo(t)

	err := repo.UpdateStock(context.Background(), 9999, 5)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("UpdateStock(9999) error = %v, want errors.Is(err, domain.ErrNotFound)", err)
	}
}

func TestStep01_UpdateStock_Fails_WhenTheNewStockIsNegative(t *testing.T) {
	// The schema has CHECK (stock >= 0): the database protects its own invariants.
	repo, _ := newRepo(t)

	err := repo.UpdateStock(context.Background(), 1, -1)

	testutil.ExpectError(t, err)
	if errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a constraint violation must not be reported as ErrNotFound: %v", err)
	}
}

func TestStep01_EveryMethod_ReturnsTheContextError_WhenTheContextIsAlreadyCancelled(t *testing.T) {
	repo, _ := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checks := map[string]error{}
	_, checks["GetByID"] = repo.GetByID(ctx, 1)
	_, checks["List"] = repo.List(ctx)
	_, checks["ListByCategory"] = repo.ListByCategory(ctx, "Audio")
	_, checks["Add"] = repo.Add(ctx, domain.Product{SKU: "X", Name: "X", PriceCents: 1, Stock: 1, Category: "X"})
	checks["UpdateStock"] = repo.UpdateStock(ctx, 1, 1)

	for name, err := range checks {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a cancelled context returned %v, want an error wrapping context.Canceled (use the ...Context methods and pass ctx)", name, err)
		}
	}
}

func TestStep01_ManyParallelReads_AllSucceed(t *testing.T) {
	repo, _ := newRepo(t)

	var wg sync.WaitGroup
	results := make([]domain.Product, 20)
	errs := make([]error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = repo.GetByID(context.Background(), int64(i+1))
		}()
	}
	wg.Wait()

	for i := range 20 {
		if errs[i] != nil {
			t.Fatalf("GetByID(%d): %v", i+1, errs[i])
		}
		if results[i].ID != int64(i+1) {
			t.Fatalf("GetByID(%d) returned product %d", i+1, results[i].ID)
		}
	}
}

func TestStep01_NoMethodLeaksAConnection(t *testing.T) {
	// With a pool of ONE connection, a method that forgets rows.Close() (or leaves a row un-scanned) keeps that connection
	// checked out forever, and the next call blocks until the timeout. Passing this test = you release everything.
	repo, d := newRepo(t)
	d.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for i := range 5 {
		if _, err := repo.GetByID(ctx, 1); err != nil {
			t.Fatalf("round %d GetByID: %v", i, err)
		}
		if _, err := repo.List(ctx); err != nil {
			t.Fatalf("round %d List: %v", i, err)
		}
		if _, err := repo.ListByCategory(ctx, "Audio"); err != nil {
			t.Fatalf("round %d ListByCategory: %v", i, err)
		}
		if err := repo.UpdateStock(ctx, 1, 10+i); err != nil {
			t.Fatalf("round %d UpdateStock: %v", i, err)
		}
	}
}
