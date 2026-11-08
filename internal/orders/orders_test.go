package orders_test

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"shopflow/internal/catalog"
	"shopflow/internal/domain"
	"shopflow/internal/events"
	"shopflow/internal/orders"
	"shopflow/internal/pricing"
	"shopflow/internal/testutil"
	"shopflow/internal/validation"
)

// Step 06 -- transactions and orchestration.   Run:  go test ./internal/orders -run Step06 -race -v
//
// NOTE: the Service tests use the REAL code from steps 1, 2, 3 and 5. Finish those first
// (or copy their reference solutions with scripts/apply-solution.sh).

var fixedNow = time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)

type recorder struct {
	mu     sync.Mutex
	events []events.OrderPlaced
}

func (r *recorder) Handle(ctx context.Context, e events.OrderPlaced) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *recorder) All() []events.OrderPlaced {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.OrderPlaced(nil), r.events...)
}

type harness struct {
	db       *sql.DB
	svc      *orders.Service
	events   *events.Publisher
	recorded *recorder
}

func newHarness(t *testing.T) harness {
	t.Helper()
	d := testutil.NewDB(t)
	products := catalog.NewSQLiteProductRepository(d)
	customers := catalog.NewSQLiteCustomerRepository(d)
	pub := events.NewPublisher()
	rec := &recorder{}
	pub.Subscribe(rec)

	clk := testutil.NewFakeClock()
	clk.Advance(fixedNow.Sub(clk.Now()))

	svc := orders.NewService(orders.Deps{
		Validate:  validation.NewOrderValidator(customers, products),
		Customers: customers,
		Products:  products,
		Pricing:   pricing.NewCalculator(pricing.BulkQuantityRule{}, pricing.LoyaltyTierRule{}),
		Orders:    orders.NewSQLiteRepository(d),
		Events:    pub,
		Clock:     clk,
	})
	return harness{db: d, svc: svc, events: pub, recorded: rec}
}

func request(customerID int64, lines ...domain.RequestedLine) domain.OrderRequest {
	return domain.OrderRequest{CustomerID: customerID, Lines: lines}
}

func line(productID int64, qty int) domain.RequestedLine {
	return domain.RequestedLine{ProductID: productID, Quantity: qty}
}

func stock(t *testing.T, d *sql.DB, productID int64) int64 {
	t.Helper()
	return testutil.Scalar(t, d, "SELECT stock FROM products WHERE id = ?", productID)
}

func codes(errs []validation.Error) []string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Code)
	}
	return out
}

// ======================= SQLiteRepository =======================

func TestStep06_Repository_Place_SavesTheOrderAndItsLines_AndTakesStock(t *testing.T) {
	d := testutil.NewDB(t)
	repo := orders.NewSQLiteRepository(d)
	ctx := context.Background()
	newOrder := orders.NewOrder{
		CustomerID: 2,
		Lines:      []orders.NewOrderLine{{ProductID: 12, Quantity: 3, UnitPriceCents: 899}, {ProductID: 1, Quantity: 2, UnitPriceCents: 8999}},
		TotalCents: 20695,
		CreatedAt:  fixedNow,
	}

	id, err := repo.Place(ctx, newOrder)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if id != 31 { // 30 seeded orders
		t.Fatalf("Place returned id %d, want 31", id)
	}

	saved, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if saved.CustomerID != 2 || saved.TotalCents != 20695 || saved.Status != domain.StatusConfirmed || !saved.CreatedAt.Equal(fixedNow) {
		t.Fatalf("saved order = %+v, want customer 2, total 20695, Confirmed, created at %v", saved, fixedNow)
	}
	wantLines := []domain.OrderLine{{ProductID: 12, Quantity: 3, UnitPriceCents: 899}, {ProductID: 1, Quantity: 2, UnitPriceCents: 8999}}
	if !slices.Equal(saved.Lines, wantLines) {
		t.Fatalf("lines = %+v, want %+v", saved.Lines, wantLines)
	}
	if got := stock(t, d, 12); got != 497 { // 500 - 3
		t.Fatalf("stock of product 12 = %d, want 497", got)
	}
	if got := stock(t, d, 1); got != 48 { // 50 - 2
		t.Fatalf("stock of product 1 = %d, want 48", got)
	}
}

func TestStep06_Repository_Place_RollsBackEverything_WhenALaterLineIsOutOfStock(t *testing.T) {
	d := testutil.NewDB(t)
	repo := orders.NewSQLiteRepository(d)
	newOrder := orders.NewOrder{
		CustomerID: 2,
		Lines:      []orders.NewOrderLine{{ProductID: 1, Quantity: 2, UnitPriceCents: 8999}, {ProductID: 15, Quantity: 1000, UnitPriceCents: 3499}}, // product 15 has 70
		TotalCents: 1,
		CreatedAt:  fixedNow,
	}

	_, err := repo.Place(context.Background(), newOrder)

	var oos *orders.OutOfStockError
	if !errors.As(err, &oos) || oos.ProductID != 15 {
		t.Fatalf("error = %v, want an *OutOfStockError for product 15 (use errors.As)", err)
	}
	if got := stock(t, d, 1); got != 50 {
		t.Fatalf("stock of product 1 = %d, want 50: the first line's reservation must be rolled back", got)
	}
	if got := stock(t, d, 15); got != 70 {
		t.Fatalf("stock of product 15 = %d, want 70", got)
	}
	if got := testutil.Scalar(t, d, "SELECT COUNT(*) FROM orders"); got != 30 {
		t.Fatalf("orders in the database = %d, want 30 (the new order must be rolled back)", got)
	}
	if got := testutil.Scalar(t, d, "SELECT COUNT(*) FROM order_lines"); got != 69 {
		t.Fatalf("order lines in the database = %d, want 69", got)
	}
}

func TestStep06_Repository_GetByID_ReturnsErrNotFound_ForAnUnknownOrder(t *testing.T) {
	repo := orders.NewSQLiteRepository(testutil.NewDB(t))

	if _, err := repo.GetByID(context.Background(), 9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestStep06_Repository_GetByID_ReadsSeededOrdersWithTheirLines(t *testing.T) {
	d := testutil.NewDB(t)

	order, err := orders.NewSQLiteRepository(d).GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetByID(1): %v", err)
	}

	if want := testutil.Scalar(t, d, "SELECT COUNT(*) FROM order_lines WHERE order_id = 1"); int64(len(order.Lines)) != want {
		t.Fatalf("order has %d lines, the database has %d", len(order.Lines), want)
	}
	if want := testutil.Scalar(t, d, "SELECT total_cents FROM orders WHERE id = 1"); order.TotalCents != want {
		t.Fatalf("total = %d, want %d", order.TotalCents, want)
	}
	var sum int64
	for _, l := range order.Lines {
		sum += l.TotalCents()
	}
	if sum != order.TotalCents { // seeded orders have no discounts
		t.Fatalf("sum of lines = %d, order total = %d", sum, order.TotalCents)
	}
}

func TestStep06_Repository_Place_ReturnsTheContextError_AndSavesNothing_WhenCancelled(t *testing.T) {
	d := testutil.NewDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := orders.NewSQLiteRepository(d).Place(ctx, orders.NewOrder{
		CustomerID: 1, Lines: []orders.NewOrderLine{{ProductID: 1, Quantity: 1, UnitPriceCents: 8999}}, TotalCents: 8999, CreatedAt: fixedNow,
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := testutil.Scalar(t, d, "SELECT COUNT(*) FROM orders"); got != 30 {
		t.Fatalf("orders = %d, want 30", got)
	}
}

// ======================= Service =======================

func TestStep06_Service_PlacesAnOrder_PricesItWithDiscounts_TakesStock_AndPublishesAnEvent(t *testing.T) {
	h := newHarness(t)

	// Customer 1 is Gold. Lines: 10 x USB-C cable (899) and 1 keyboard (8999).
	// subtotal 17989; bulk = 10% of 8990 = 899; loyalty = 5% of 17989 = 899 (floor) -> total 16191.
	res, err := h.svc.PlaceOrder(context.Background(), request(1, line(12, 10), line(1, 1)))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if !res.Succeeded() {
		t.Fatalf("PlaceOrder failed with %v", res.Errors)
	}

	order := res.Order
	if order.TotalCents != 16191 || order.Status != domain.StatusConfirmed || !order.CreatedAt.Equal(fixedNow) {
		t.Fatalf("order = %+v, want total 16191, Confirmed, created at %v", *order, fixedNow)
	}
	wantLines := []domain.OrderLine{{ProductID: 12, Quantity: 10, UnitPriceCents: 899}, {ProductID: 1, Quantity: 1, UnitPriceCents: 8999}}
	if !slices.Equal(order.Lines, wantLines) { // unit prices are undiscounted snapshots
		t.Fatalf("lines = %+v, want %+v", order.Lines, wantLines)
	}
	if got := stock(t, h.db, 12); got != 490 {
		t.Fatalf("stock of product 12 = %d, want 490", got)
	}
	if got := stock(t, h.db, 1); got != 49 {
		t.Fatalf("stock of product 1 = %d, want 49", got)
	}

	published := h.recorded.All()
	if want := (events.OrderPlaced{OrderID: order.ID, CustomerID: 1, TotalCents: 16191}); len(published) != 1 || published[0] != want {
		t.Fatalf("published events = %+v, want exactly %+v", published, want)
	}
}

func TestStep06_Service_RejectsAnInactiveCustomer_AndChangesNothing(t *testing.T) {
	h := newHarness(t)

	res, err := h.svc.PlaceOrder(context.Background(), request(5, line(1, 1))) // customer 5 is inactive
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if res.Succeeded() || !slices.Equal(codes(res.Errors), []string{validation.CodeCustomerInactive}) {
		t.Fatalf("result = %+v, want a failure with CUSTOMER_INACTIVE", res)
	}
	if got := testutil.Scalar(t, h.db, "SELECT COUNT(*) FROM orders"); got != 30 {
		t.Fatalf("orders = %d, want 30", got)
	}
	if got := stock(t, h.db, 1); got != 50 {
		t.Fatalf("stock = %d, want 50", got)
	}
	if len(h.recorded.All()) != 0 {
		t.Fatal("no event should be published for a rejected order")
	}
}

func TestStep06_Service_RejectsAnOrder_WhenThereIsNotEnoughStock(t *testing.T) {
	h := newHarness(t)

	res, err := h.svc.PlaceOrder(context.Background(), request(1, line(4, 16))) // product 4 has 15 in stock
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if res.Succeeded() || !slices.Equal(codes(res.Errors), []string{validation.CodeInsufficientStock}) {
		t.Fatalf("result = %+v, want a failure with INSUFFICIENT_STOCK", res)
	}
	if got := stock(t, h.db, 4); got != 15 {
		t.Fatalf("stock = %d, want 15", got)
	}
}

func TestStep06_Service_RejectsAnOrder_WhenTheCreditLimitWouldBeExceeded(t *testing.T) {
	h := newHarness(t)

	// Customer 6 has a 20000-cent limit; one 4K monitor costs 39999.
	res, err := h.svc.PlaceOrder(context.Background(), request(6, line(4, 1)))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if res.Succeeded() || !slices.Equal(codes(res.Errors), []string{validation.CodeCreditLimitExceeded}) {
		t.Fatalf("result = %+v, want a failure with CREDIT_LIMIT_EXCEEDED", res)
	}
}

func TestStep06_Service_StillSucceeds_WhenAnEventHandlerFails(t *testing.T) {
	h := newHarness(t)
	h.events.Subscribe(events.HandlerFunc(func(context.Context, events.OrderPlaced) error {
		return errors.New("notification system is down")
	}))

	res, err := h.svc.PlaceOrder(context.Background(), request(1, line(1, 1)))

	if err != nil || !res.Succeeded() { // the order is committed; a broken notifier must not turn it into a failure
		t.Fatalf("PlaceOrder = (%+v, %v), want success", res, err)
	}
	if got := testutil.Scalar(t, h.db, "SELECT COUNT(*) FROM orders"); got != 31 {
		t.Fatalf("orders = %d, want 31", got)
	}
	if len(h.recorded.All()) != 1 {
		t.Fatal("the healthy handler should still have been notified")
	}
}

func TestStep06_Service_ReturnsInfrastructureErrorsAsErrors_NotAsValidationFailures(t *testing.T) {
	dbDown := errors.New("database down")
	svc := orders.NewService(orders.Deps{
		Validate: func(context.Context, domain.OrderRequest) ([]validation.Error, error) { return nil, dbDown },
	})

	res, err := svc.PlaceOrder(context.Background(), request(1, line(1, 1)))

	if !errors.Is(err, dbDown) {
		t.Fatalf("error = %v, want the infrastructure error", err)
	}
	if res.Succeeded() || len(res.Errors) != 0 {
		t.Fatalf("result = %+v, want an empty failure", res)
	}
}

func TestStep06_Service_WhenManyBuyersRaceForTheLastUnit_ExactlyOneWins(t *testing.T) {
	h := newHarness(t)
	testutil.Exec(t, h.db, `INSERT INTO products (id, sku, name, price_cents, stock, category) VALUES (100, 'LAST-ONE', 'Last One', 1000, 1, 'Rare')`)

	const buyers = 6
	results := make([]orders.Result, buyers)
	errs := make([]error, buyers)
	var wg sync.WaitGroup
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = h.svc.PlaceOrder(context.Background(), request(1, line(100, 1)))
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	testutil.WaitClosed(t, done, 20*time.Second)

	wins := 0
	for i := range buyers {
		if errs[i] != nil {
			t.Fatalf("buyer %d got an error: %v", i, errs[i])
		}
		if results[i].Succeeded() {
			wins++
		} else if !slices.Equal(codes(results[i].Errors), []string{validation.CodeInsufficientStock}) {
			t.Fatalf("buyer %d lost with %v, want INSUFFICIENT_STOCK", i, results[i].Errors)
		}
	}
	if wins != 1 {
		t.Fatalf("%d buyers got the last unit, want exactly 1", wins)
	}
	if got := stock(t, h.db, 100); got != 0 {
		t.Fatalf("stock = %d, want 0 (never negative, never oversold)", got)
	}
	if got := testutil.Scalar(t, h.db, "SELECT COUNT(*) FROM orders"); got != 31 {
		t.Fatalf("orders = %d, want 31", got)
	}
	if got := testutil.Scalar(t, h.db, "SELECT COUNT(*) FROM orders WHERE total_cents = 950"); got != 1 { // Gold: 5% off 1000
		t.Fatalf("orders with total 950 = %d, want 1", got)
	}
}

func TestStep06_Service_ReturnsAnError_WhenTheContextIsAlreadyCancelled(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.svc.PlaceOrder(ctx, request(1, line(1, 1)))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := testutil.Scalar(t, h.db, "SELECT COUNT(*) FROM orders"); got != 30 {
		t.Fatalf("orders = %d, want 30", got)
	}
}
