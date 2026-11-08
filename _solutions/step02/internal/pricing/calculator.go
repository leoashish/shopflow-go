package pricing

type Calculator struct {
	rules []Rule
}

func NewCalculator(rules ...Rule) *Calculator {
	return &Calculator{rules: append([]Rule(nil), rules...)} // copy: later changes to the caller's slice can't affect us
}

func (c *Calculator) Calculate(cart Cart) Breakdown {
	subtotal := cart.SubtotalCents()
	budget := subtotal * MaxTotalDiscountPercent / 100
	var applied []AppliedDiscount
	var granted int64

	for _, rule := range c.rules {
		wanted := max(0, rule.DiscountCents(cart)) // builtin max, Go 1.21+
		amount := min(wanted, budget)
		if amount <= 0 {
			continue
		}
		applied = append(applied, AppliedDiscount{Rule: rule.Name(), AmountCents: amount})
		budget -= amount
		granted += amount
	}

	return Breakdown{SubtotalCents: subtotal, Discounts: applied, TotalCents: subtotal - granted}
}
