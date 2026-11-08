# Step 03 — Order validation as a middleware chain

| | |
|---|---|
| **Build** | `Chain` (`chain.go`), `CustomerEligibility`, `OrderLines`, `StockAvailability`, `CreditLimit` (`checks.go`), `NewOrderValidator` (`validator.go`) — all in `internal/validation/` |
| **Tests** | `internal/validation/validation_test.go` — 25 tests |
| **Run** | `go test ./internal/validation -run Step03 -v` |
| **You'll learn** | Chain of Responsibility via function types and closures, higher-order functions, Interface Segregation the Go way, business-vs-infrastructure errors, fakes without a mocking library |
| **Time** | ~60–90 min |

## 1. The story

Before an order is accepted we must check a lot: is the customer allowed to buy, is the order well-formed, is everything in stock, can they afford it? One 200-line function is something nobody dares touch. Instead each check becomes a small **middleware**, and the middlewares are linked into a **chain**.

## 2. Concept corner

**Chain of Responsibility with function types.** In Go the natural form is the one `net/http` middleware uses:

```go
type Handler    func(ctx context.Context, req domain.OrderRequest) ([]Error, error)
type Middleware func(next Handler) Handler
```

A middleware *wraps* the rest of the chain. It does its check; if it fails it answers **right there without calling `next`**; otherwise it calls `next`:

```go
func SomeCheck() Middleware {
    return func(next Handler) Handler {            // called once, when the chain is built
        return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {   // called per request
            if problem { return []Error{{Code: "X"}}, nil }
            return next(ctx, req)
        }
    }
}
```

Functions that take or return functions are *higher-order*; the inner function **closes over** `next` (and any config such as a repository) — that is a closure, and it replaces the base class + fields you'd write in C#.

**Building the chain.** `Chain(a, b, c)` must behave like `a(b(c(end)))`, with the *first* middleware running first. Build it from the inside out: start with the terminal handler, loop over the middlewares **backwards**, wrapping what you have so far.

**Two kinds of bad news — keep them apart.**
- `[]Error` is *business validation* ("customer inactive"): the request is wrong; the system is fine. Empty/nil ⇒ valid.
- `error` is *infrastructure trouble* ("database down"): we could not decide at all.

If you turn a database outage into a `CUSTOMER_NOT_FOUND` validation error you will reject real customers during an outage. Tests check that outages pass through as errors.

**Interface Segregation falls out of Go.** Look at `validation.go`: the checks declare `CustomerGetter` and `ProductGetter` — one method each — *themselves*. `catalog.ProductRepository` has five methods; nothing here needs four of them. Any type with `GetByID` qualifies, with no "implements" keyword, so the test fakes in `testutil` just work. *Define interfaces where they are used, as small as the user needs.*

**Fail-fast vs collect-all.** *Between* middlewares: fail-fast (if the customer is inactive, don't bother the database about stock). *Within* a middleware: collect everything (tell the user about all three bad lines at once).

## 3. Requirements

1. **`Chain(mws...)`** — first passed runs first; each can stop the chain; the end of the chain is `(nil, nil)`; `Chain()` is just that end. The context reaches every middleware.
2. **`CustomerEligibility(customers)`** — not found (`errors.Is(err, domain.ErrNotFound)`) ⇒ one `CodeCustomerNotFound`; inactive ⇒ one `CodeCustomerInactive`; any *other* error ⇒ `return nil, err`.
3. **`OrderLines()`** (no I/O) — collect **all**: no lines ⇒ one `CodeEmptyOrder`; each line with quantity < 1 or > `MaxQuantityPerLine` ⇒ one `CodeInvalidQuantity` **per bad line**; each product id on more than one line ⇒ one `CodeDuplicateProduct` **per duplicated product**.
4. **`StockAvailability(products)`** — for each line: not found ⇒ `CodeProductNotFound`; `Stock < quantity` ⇒ `CodeInsufficientStock` (exactly enough is fine); other errors ⇒ `return nil, err`. Report **all** failing lines; if none, call `next`.
5. **`CreditLimit(customers, products)`** — total of `price × quantity` over all lines (no discounts) must be ≤ the customer's credit limit, else one `CodeCreditLimitExceeded`. Equal is fine; one cent over is not. A missing customer or product is skipped (earlier checks report it). Other errors ⇒ `(nil, err)`.
6. **`NewOrderValidator(customers, products)`** — `CustomerEligibility → OrderLines → StockAvailability → CreditLimit`.

## 4. What the tests pin down

Chain mechanics with *scripted* middlewares (order, stop-at-first-failure, empty chain, infrastructure error propagation, context reaches the end); each check's pass/fail cases and boundaries (100 vs 101, exact credit, +1 cent); error collection; outages stay errors; and the assembled chain's *ordering* (inactive customer beats empty order; lines before stock; stock before credit).

## 5. Hints

<details><summary>Hint 1 — Chain is five lines</summary>

```go
var h Handler = func(ctx context.Context, req domain.OrderRequest) ([]Error, error) { return nil, nil }
for i := len(mws) - 1; i >= 0; i-- { h = mws[i](h) }
return h
```
</details>

<details><summary>Hint 2 — each check has the same skeleton</summary>

`return func(next Handler) Handler { return func(ctx, req) ([]Error, error) { ...; return next(ctx, req) } }`. Replace the `notImplemented(...)` call in `checks.go` with that.
</details>

<details><summary>Hint 3 — spotting duplicates in order</summary>

A `map[int64]int` of counts; when a count *becomes* 2, append that product id to a `duplicates` slice — that keeps the output order deterministic (ranging over a map is random in Go).
</details>

<details><summary>Hint 4 — sentinel vs infrastructure</summary>

```go
c, err := customers.GetByID(ctx, req.CustomerID)
if errors.Is(err, domain.ErrNotFound) { return []Error{...}, nil }
if err != nil { return nil, err }
```
</details>

## 6. Stretch challenges

1. A **logging/timing middleware** that wraps *any* chain and records how long each stage took — notice you don't touch the checks.
2. A `MaxOrderValue(limit)` middleware ("over $5,000 needs approval ⇒ `NEEDS_APPROVAL`"): one new function and one line in `NewOrderValidator`.
3. `StockAvailability` and `CreditLimit` both load every product, so each product is fetched twice per request. Fix it with Step 04's cache — or by passing a per-request cache through the context.
4. Add a **collect-everything mode** that runs all middlewares and merges their errors. What signature does a middleware need then?
5. Run the independent checks concurrently with Step 09's `concurrency.Map`. Which checks are independent? Is it worth it?

## 7. Reflect

- Where do you see the Single Responsibility Principle in this design, compared with one big `Validate` function?
- Chain of Responsibility vs. a plain `[]Check` slice looped over: practical differences? When is each better?
- Why is `(nil, nil)` a *valid* return here, when Go style usually frowns on it?
- What happens to the chain if a middleware panics? Should it?

## 8. Done when

`go test ./internal/validation -run Step03` → **ok**.
