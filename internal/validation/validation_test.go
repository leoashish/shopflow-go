package validation_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"shopflow/internal/domain"
	"shopflow/internal/testutil"
	"shopflow/internal/validation"
)

// Step 03 -- Chain of Responsibility with middleware.   Run:  go test ./internal/validation -run Step03 -v

func req(customerID int64, lines ...domain.RequestedLine) domain.OrderRequest {
	return domain.OrderRequest{CustomerID: customerID, Lines: lines}
}

func line(productID int64, qty int) domain.RequestedLine {
	return domain.RequestedLine{ProductID: productID, Quantity: qty}
}

func codes(errs []validation.Error) []string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Code)
	}
	return out
}

// run calls the handler and fails the test on an infrastructure error, returning only the business errors' codes.
func run(t *testing.T, h validation.Handler, r domain.OrderRequest) []string {
	t.Helper()
	errs, err := h(context.Background(), r)
	if err != nil {
		t.Fatalf("handler returned an infrastructure error: %v", err)
	}
	return codes(errs)
}

func assertCodes(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("error codes = %v, want %v", got, want)
	}
}

// scripted is a middleware whose outcome the test decides, so we can test the CHAIN MECHANICS on their own.
func scripted(name string, calls *[]string, errs []validation.Error, err error) validation.Middleware {
	return func(next validation.Handler) validation.Handler {
		return func(ctx context.Context, r domain.OrderRequest) ([]validation.Error, error) {
			*calls = append(*calls, name)
			if len(errs) > 0 || err != nil {
				return errs, err
			}
			return next(ctx, r)
		}
	}
}

// ======================= Chain =======================

func TestStep03_Chain_RunsTheMiddlewaresInOrder_ThenReportsValid(t *testing.T) {
	var calls []string
	h := validation.Chain(scripted("a", &calls, nil, nil), scripted("b", &calls, nil, nil), scripted("c", &calls, nil, nil))

	errs, err := h(context.Background(), req(1, line(1, 1)))

	if err != nil || len(errs) != 0 {
		t.Fatalf("got (%v, %v), want a valid result", errs, err)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(calls, want) {
		t.Fatalf("middlewares ran in order %v, want %v (the FIRST one passed must run first)", calls, want)
	}
}

func TestStep03_Chain_StopsAtTheFirstFailure_AndReturnsItsErrors(t *testing.T) {
	var calls []string
	boom := []validation.Error{{Code: "BOOM", Message: "b failed"}}
	h := validation.Chain(scripted("a", &calls, nil, nil), scripted("b", &calls, boom, nil), scripted("c", &calls, nil, nil))

	got := run(t, h, req(1, line(1, 1)))

	assertCodes(t, got, "BOOM")
	if want := []string{"a", "b"}; !slices.Equal(calls, want) {
		t.Fatalf("middlewares that ran = %v, want %v (c must not run after b failed)", calls, want)
	}
}

func TestStep03_Chain_WithNoMiddlewares_IsValid(t *testing.T) {
	errs, err := validation.Chain()(context.Background(), req(1))
	if err != nil || len(errs) != 0 {
		t.Fatalf("got (%v, %v), want valid", errs, err)
	}
}

func TestStep03_Chain_PropagatesInfrastructureErrors_AndStops(t *testing.T) {
	var calls []string
	dbDown := errors.New("database down")
	h := validation.Chain(scripted("a", &calls, nil, dbDown), scripted("b", &calls, nil, nil))

	errs, err := h(context.Background(), req(1, line(1, 1)))

	if !errors.Is(err, dbDown) {
		t.Fatalf("error = %v, want the infrastructure error", err)
	}
	if len(errs) != 0 || len(calls) != 1 {
		t.Fatalf("validation errors = %v, calls = %v; want none and only 'a' called", errs, calls)
	}
}

type ctxKey struct{}

func TestStep03_Chain_PassesTheContextAlongTheWholeChain(t *testing.T) {
	var seen any
	probe := func(next validation.Handler) validation.Handler {
		return func(ctx context.Context, r domain.OrderRequest) ([]validation.Error, error) {
			seen = ctx.Value(ctxKey{})
			return next(ctx, r)
		}
	}
	var calls []string
	h := validation.Chain(scripted("a", &calls, nil, nil), probe)

	ctx := context.WithValue(context.Background(), ctxKey{}, "hello")
	if _, err := h(ctx, req(1, line(1, 1))); err != nil {
		t.Fatal(err)
	}

	if seen != "hello" {
		t.Fatalf("the last middleware saw context value %v, want %q", seen, "hello")
	}
}

// ======================= CustomerEligibility =======================

func TestStep03_Customer_PassesForAnActiveCustomer_AndCallsNext(t *testing.T) {
	customers := testutil.FakeCustomers{1: testutil.NewCustomer(1, domain.TierStandard, true, 1000)}
	var calls []string
	h := validation.Chain(validation.CustomerEligibility(customers), scripted("next", &calls, nil, nil))

	assertCodes(t, run(t, h, req(1, line(1, 1))))
	if !slices.Equal(calls, []string{"next"}) {
		t.Fatalf("next was not called: %v", calls)
	}
}

func TestStep03_Customer_Fails_WhenTheCustomerDoesNotExist(t *testing.T) {
	h := validation.Chain(validation.CustomerEligibility(testutil.FakeCustomers{}))
	assertCodes(t, run(t, h, req(42, line(1, 1))), validation.CodeCustomerNotFound)
}

func TestStep03_Customer_Fails_WhenTheCustomerIsInactive(t *testing.T) {
	h := validation.Chain(validation.CustomerEligibility(testutil.FakeCustomers{1: testutil.NewCustomer(1, domain.TierStandard, false, 1000)}))
	assertCodes(t, run(t, h, req(1, line(1, 1))), validation.CodeCustomerInactive)
}

type brokenCustomers struct{ err error }

func (b brokenCustomers) GetByID(context.Context, int64) (domain.Customer, error) {
	return domain.Customer{}, b.err
}

func TestStep03_Customer_ReportsInfrastructureErrorsAsErrors_NotAsValidationFailures(t *testing.T) {
	dbDown := errors.New("database down")
	h := validation.Chain(validation.CustomerEligibility(brokenCustomers{err: dbDown}))

	errs, err := h(context.Background(), req(1, line(1, 1)))

	if !errors.Is(err, dbDown) {
		t.Fatalf("error = %v, want the database error", err)
	}
	if len(errs) != 0 {
		t.Fatalf("an outage must not look like a validation failure, got %v", errs)
	}
}

// ======================= OrderLines =======================

func TestStep03_Lines_Fails_WhenTheOrderHasNoLines(t *testing.T) {
	assertCodes(t, run(t, validation.Chain(validation.OrderLines()), req(1)), validation.CodeEmptyOrder)
}

func TestStep03_Lines_EnforcesAQuantityBetweenOneAndOneHundred(t *testing.T) {
	tests := []struct {
		qty   int
		valid bool
	}{{-1, false}, {0, false}, {1, true}, {100, true}, {101, false}}
	for _, tc := range tests {
		got := run(t, validation.Chain(validation.OrderLines()), req(1, line(1, tc.qty)))
		if tc.valid {
			assertCodes(t, got)
		} else {
			assertCodes(t, got, validation.CodeInvalidQuantity)
		}
	}
}

func TestStep03_Lines_Fails_WhenTheSameProductAppearsTwice(t *testing.T) {
	got := run(t, validation.Chain(validation.OrderLines()), req(1, line(5, 1), line(5, 2), line(6, 1)))
	assertCodes(t, got, validation.CodeDuplicateProduct)
}

func TestStep03_Lines_CollectsEveryProblem_NotJustTheFirst(t *testing.T) {
	// product 1 has an invalid quantity, product 2 is duplicated, product 3 is fine.
	got := run(t, validation.Chain(validation.OrderLines()), req(1, line(1, 0), line(2, 1), line(2, 1), line(3, 1)))

	if len(got) != 2 || !slices.Contains(got, validation.CodeInvalidQuantity) || !slices.Contains(got, validation.CodeDuplicateProduct) {
		t.Fatalf("error codes = %v, want one INVALID_QUANTITY and one DUPLICATE_PRODUCT", got)
	}
}

// ======================= StockAvailability =======================

func TestStep03_Stock_Passes_WhenThereIsExactlyEnoughStock(t *testing.T) {
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 100, 5))
	assertCodes(t, run(t, validation.Chain(validation.StockAvailability(products)), req(1, line(1, 5))))
}

func TestStep03_Stock_ReportsEveryLineThatIsShort(t *testing.T) {
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 100, 5), testutil.NewProduct(2, 100, 1), testutil.NewProduct(3, 100, 10))

	got := run(t, validation.Chain(validation.StockAvailability(products)), req(1, line(1, 6), line(2, 2), line(3, 10)))

	assertCodes(t, got, validation.CodeInsufficientStock, validation.CodeInsufficientStock)
}

func TestStep03_Stock_Fails_WhenTheProductDoesNotExist(t *testing.T) {
	got := run(t, validation.Chain(validation.StockAvailability(testutil.NewFakeProducts())), req(1, line(99, 1)))
	assertCodes(t, got, validation.CodeProductNotFound)
}

type brokenProducts struct{ err error }

func (b brokenProducts) GetByID(context.Context, int64) (domain.Product, error) {
	return domain.Product{}, b.err
}

func TestStep03_Stock_ReportsInfrastructureErrorsAsErrors(t *testing.T) {
	dbDown := errors.New("database down")
	_, err := validation.Chain(validation.StockAvailability(brokenProducts{err: dbDown}))(context.Background(), req(1, line(1, 1)))
	if !errors.Is(err, dbDown) {
		t.Fatalf("error = %v, want the database error", err)
	}
}

// ======================= CreditLimit =======================

func TestStep03_Credit_Passes_WhenTheTotalEqualsTheLimit(t *testing.T) {
	customers := testutil.FakeCustomers{1: testutil.NewCustomer(1, domain.TierStandard, true, 3000)}
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 1000, 100))
	assertCodes(t, run(t, validation.Chain(validation.CreditLimit(customers, products)), req(1, line(1, 3))))
}

func TestStep03_Credit_Fails_WhenTheTotalIsOneCentOverTheLimit(t *testing.T) {
	customers := testutil.FakeCustomers{1: testutil.NewCustomer(1, domain.TierStandard, true, 2999)}
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 1000, 100))
	assertCodes(t, run(t, validation.Chain(validation.CreditLimit(customers, products)), req(1, line(1, 3))), validation.CodeCreditLimitExceeded)
}

func TestStep03_Credit_IgnoresMissingCustomersAndProducts_BecauseOtherChecksReportThose(t *testing.T) {
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 1000, 100))
	assertCodes(t, run(t, validation.Chain(validation.CreditLimit(testutil.FakeCustomers{}, products)), req(7, line(1, 3))))

	customers := testutil.FakeCustomers{1: testutil.NewCustomer(1, domain.TierStandard, true, 10)}
	assertCodes(t, run(t, validation.Chain(validation.CreditLimit(customers, testutil.NewFakeProducts())), req(1, line(99, 3))))
}

// ======================= NewOrderValidator (the whole chain) =======================

func validator() validation.Handler {
	customers := testutil.FakeCustomers{
		1: testutil.NewCustomer(1, domain.TierStandard, true, 100_000),
		2: testutil.NewCustomer(2, domain.TierStandard, false, 100_000),
		3: testutil.NewCustomer(3, domain.TierStandard, true, 1_000),
	}
	products := testutil.NewFakeProducts(testutil.NewProduct(1, 1_000, 5), testutil.NewProduct(2, 50_000, 10))
	return validation.NewOrderValidator(customers, products)
}

func TestStep03_Validator_AcceptsAValidOrder(t *testing.T) {
	assertCodes(t, run(t, validator(), req(1, line(1, 2), line(2, 1))))
}

func TestStep03_Validator_ReportsOnlyTheFirstFailingStage_InactiveCustomerBeatsEverythingElse(t *testing.T) {
	// inactive customer AND an empty order: the customer check runs first, so ONLY that error is reported.
	assertCodes(t, run(t, validator(), req(2)), validation.CodeCustomerInactive)
}

func TestStep03_Validator_ChecksLinesBeforeStock(t *testing.T) {
	assertCodes(t, run(t, validator(), req(1, line(1, 0))), validation.CodeInvalidQuantity)
}

func TestStep03_Validator_ChecksStockBeforeCredit(t *testing.T) {
	// 6 x product 1 is more than the 5 in stock (and customer 3's tiny credit limit would fail too).
	assertCodes(t, run(t, validator(), req(3, line(1, 6))), validation.CodeInsufficientStock)
}

func TestStep03_Validator_FinallyChecksCredit(t *testing.T) {
	// Customer 3 has 1000 credit: one unit of product 1 costs exactly 1000 (ok), two units don't fit (stock is fine).
	assertCodes(t, run(t, validator(), req(3, line(1, 1))))
	assertCodes(t, run(t, validator(), req(3, line(1, 2))), validation.CodeCreditLimitExceeded)
}
