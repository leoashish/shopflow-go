package pricing

// Calculator -- STEP 02. Applies discount rules IN ORDER and produces a Breakdown.
//
//  1. Each rule is evaluated against the original cart (rules do not see each other's discounts).
//  2. Total discounts may never exceed MaxTotalDiscountPercent of the subtotal (round the cap down). Keep a
//     "remaining budget"; a rule's discount is truncated to what is left. Rules are processed in the order given,
//     so order matters.
//  3. A rule that contributes 0 (or a negative number -- treat it as 0) is NOT listed in the breakdown.
//  4. TotalCents = SubtotalCents - the sum of the applied discounts.
type Calculator struct {
	rules []Rule
}

func NewCalculator(rules ...Rule) *Calculator {
	// TODO Step 02: keep a copy of the rules
	return &Calculator{}
}

func (c *Calculator) Calculate(cart Cart) Breakdown {
	return Breakdown{} // TODO Step 02
}
