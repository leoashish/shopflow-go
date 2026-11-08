// Package pricing turns a shopping cart into a price, applying discount rules.
package pricing

import (
	"errors"

	"shopflow/internal/domain"
)

// ErrInvalidPercent is returned when a percentage is outside 1..100.
var ErrInvalidPercent = errors.New("percentOff must be between 1 and 100")

// MaxTotalDiscountPercent caps the total discount at this share of the subtotal.
const MaxTotalDiscountPercent = 30

// CartLine is a product in the cart with a quantity.
type CartLine struct {
	Product  domain.Product
	Quantity int
}

func (l CartLine) TotalCents() int64 { return l.Product.PriceCents * int64(l.Quantity) }

// Cart is everything a discount rule is allowed to look at.
type Cart struct {
	Customer domain.Customer
	Lines    []CartLine
}

func (c Cart) SubtotalCents() int64 {
	var sum int64
	for _, l := range c.Lines {
		sum += l.TotalCents()
	}
	return sum
}

type AppliedDiscount struct {
	Rule        string
	AmountCents int64
}

type Breakdown struct {
	SubtotalCents int64
	Discounts     []AppliedDiscount
	TotalCents    int64
}

func (b Breakdown) TotalDiscountCents() int64 {
	var sum int64
	for _, d := range b.Discounts {
		sum += d.AmountCents
	}
	return sum
}

// Rule is the STRATEGY interface: one small, swappable pricing rule.
// Adding a new rule means adding a new type -- the Calculator never changes (Open/Closed Principle).
type Rule interface {
	Name() string

	// DiscountCents is how many cents this rule would take off, judged against the ORIGINAL cart.
	// It returns 0 when the rule does not apply. All math is integer math: percentages round DOWN.
	DiscountCents(cart Cart) int64
}

// RuleFunc turns a plain function into a Rule -- the same trick as http.HandlerFunc. (GIVEN.)
// It lets you define a one-off rule inline, e.g. in a test:
//
//	pricing.RuleFunc("Coupon", func(pricing.Cart) int64 { return 700 })
func RuleFunc(name string, fn func(Cart) int64) Rule { return funcRule{name: name, fn: fn} }

type funcRule struct {
	name string
	fn   func(Cart) int64
}

func (r funcRule) Name() string                  { return r.name }
func (r funcRule) DiscountCents(cart Cart) int64 { return r.fn(cart) }
