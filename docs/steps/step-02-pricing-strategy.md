# Step 02 — Pricing rules with the Strategy pattern

| | |
|---|---|
| **Build** | `BulkQuantityRule`, `LoyaltyTierRule`, `CategoryPromotionRule` (`internal/pricing/rules.go`) and `Calculator` (`calculator.go`) |
| **Tests** | `internal/pricing/pricing_test.go` — 16 tests |
| **Run** | `go test ./internal/pricing -run Step02 -v` |
| **You'll learn** | Strategy via interfaces, function adapters (`RuleFunc`), value vs pointer receivers, slice aliasing, integer money math, why rule order matters |
| **Time** | ~45–60 min (no database, no concurrency — pure logic) |

## 1. The story

Marketing changes the discount rules every month: "10% off 10+ units", "Gold members get 5%", "20% off Audio this week". If price calculation is one giant function full of `if`s, every campaign means editing — and risking — the same fragile code.

## 2. Concept corner

**Strategy pattern, Go style.** Put each interchangeable algorithm behind a small interface and let a coordinator use them without knowing which exist:

```go
type Rule interface {
    Name() string
    DiscountCents(cart Cart) int64
}
// Calculator loops over []Rule. A new rule is a new type; Calculator never changes.
```

**Open/Closed Principle.** Open for extension, closed for modification. The test `Calculator_AcceptsBrandNewRules_WithoutModification` defines a rule *inline* with `pricing.RuleFunc("Coupon", func(pricing.Cart) int64 { return 700 })` and plugs it in.

**Function adapters.** `RuleFunc` (given) turns a plain function into a `Rule` — the same trick as `http.HandlerFunc`. In Go, *any named type can have methods*, including function types:

```go
type funcRule struct{ name string; fn func(Cart) int64 }
func (r funcRule) Name() string                  { return r.name }
func (r funcRule) DiscountCents(c Cart) int64    { return r.fn(c) }
```

Read how it is built in `pricing.go`; you will meet this pattern all over the standard library.

**Value vs pointer receivers.** `BulkQuantityRule` and `LoyaltyTierRule` have no state, so a value receiver is natural and `pricing.BulkQuantityRule{}` *is* a `Rule`. `CategoryPromotionRule` carries configuration and must be validated, so it is built by a constructor that returns `(*CategoryPromotionRule, error)` and its methods use a pointer receiver. Rule of thumb: if a constructor can fail or the type holds state, return a pointer.

**Money = integers.** `0.1 + 0.2 != 0.3` in floating point. Money is `int64` **cents**; percentages are `amount * percent / 100` (multiply first, divide last) and integer division **rounds down** — our documented rounding rule.

**Slice aliasing.** A slice is a *view* of an array. `NewCalculator(rules...)` receives the caller's slice; if you store it as-is, the caller can later change `rules[0]` and silently change your calculator. A test does exactly that. Copy: `append([]Rule(nil), rules...)`.

**Order can matter.** With a cap (total discount ≤ 30 % of the subtotal), whichever rule is processed first gets first claim on the budget. The tests show the same cart priced differently with the rules reversed.

## 3. Requirements

**Given** (read them first): `Cart`, `CartLine`, `AppliedDiscount`, `Breakdown`, the `Rule` interface, `RuleFunc`, `ErrInvalidPercent`, `MaxTotalDiscountPercent`.

| Type | `Name()` | Rule |
|---|---|---|
| `BulkQuantityRule` | `"BulkQuantity"` | Every line with `Quantity >= 10` gets **10 % off that line's total** (use the constants). Sum the lines; round each down. |
| `LoyaltyTierRule` | `"Loyalty"` | % of the whole **subtotal** by tier: Standard 0 %, Silver 3 %, Gold 5 %. Round down. |
| `CategoryPromotionRule` (`NewCategoryPromotionRule(category, percentOff)`) | `"Category:" + category` | `percentOff`% off the total of lines whose product category equals `category` (**case-insensitive**: `strings.EqualFold`). `percentOff` outside 1–100 ⇒ `ErrInvalidPercent`. |

**`Calculator.Calculate(cart)`**

1. Each rule is evaluated against the **original** cart.
2. Total discounts never exceed `MaxTotalDiscountPercent` (30 %) of the subtotal (round the cap down). Keep a *remaining budget*; truncate each rule's discount to what is left. Rules are processed **in the order given**.
3. A rule contributing 0 — or a negative number (treat as 0) — is **not listed** in the breakdown.
4. `TotalCents = SubtotalCents − applied discounts`. An empty cart is fine (all zeros).
5. The calculator keeps its **own copy** of the rules.

## 4. What the tests pin down

Exact cent values per rule (including round-down cases and the boundary at quantity 10), tier table, case-insensitive categories, percent validation, discounts listed in rule order, zero-discount rules omitted, the 30 % cap in both rule orders, an inline `RuleFunc` rule, negative discounts ignored, the empty cart, and the slice-aliasing trap.

## 5. Hints

<details><summary>Hint 1 — three shapes</summary>

`Bulk`: loop over lines, `if line.Quantity >= BulkMinimumQuantity { total += line.TotalCents() * BulkPercentOff / 100 }`. `Loyalty`: `switch cart.Customer.Tier`. `Category`: sum matching lines first, then take the percentage.
</details>

<details><summary>Hint 2 — the budget loop</summary>

```text
budget = subtotal * 30 / 100
for each rule in order:
    wanted  = max(0, rule.DiscountCents(cart))     // builtin max/min since Go 1.21
    granted = min(wanted, budget)
    if granted > 0 { record it; budget -= granted }
```
</details>

<details><summary>Hint 3 — why does a test break when I store `rules` directly?</summary>

See "Slice aliasing" above. `append([]Rule(nil), rules...)` makes an independent copy.
</details>

## 6. Stretch challenges

1. **Buy 2, get the cheapest free** — write the failing test first, then the type. Notice you touch *no* existing production code.
2. A `CouponRule` with an expiry date; inject a `clock.Clock` so the test controls "now".
3. Make the cap configurable: `NewCalculator(opts, rules...)` — or use functional options (`WithMaxDiscountPercent(25)`).
4. Let each rule declare a `Priority()` and sort by it. Better or worse than "the caller decides the order"? Argue both sides.

## 7. Reflect

- Where did the `if`/`switch` chains go? `LoyaltyTierRule` still has a `switch`: is that an Open/Closed violation? What would you do if tiers kept growing?
- Why does `Calculator` accept `...Rule` instead of constructing the rules itself?
- Why can `BulkQuantityRule{}` be used as a `Rule` but you need `&CategoryPromotionRule{}` (a pointer) for the other?
- In `float64`, `0.1 + 0.2 == 0.3` is `false`. What could that do to an order total that must match the invoice to the cent?

## 8. Done when

`go test ./internal/pricing -run Step02` → **ok**.
