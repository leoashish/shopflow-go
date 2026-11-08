package pricing_test

import (
	"errors"
	"slices"
	"testing"

	"shopflow/internal/domain"
	"shopflow/internal/pricing"
)

// Step 02 -- Strategy pattern + Open/Closed.   Run:  go test ./internal/pricing -run Step02 -v

func line(price int64, qty int, category string, id int64) pricing.CartLine {
	return pricing.CartLine{
		Product:  domain.Product{ID: id, SKU: "SKU", Name: "Item", PriceCents: price, Stock: 100, Category: category},
		Quantity: qty,
	}
}

func cart(tier domain.CustomerTier, lines ...pricing.CartLine) pricing.Cart {
	return pricing.Cart{Customer: domain.Customer{ID: 1, Tier: tier, Active: true}, Lines: lines}
}

func mustPromo(t *testing.T, category string, percent int) *pricing.CategoryPromotionRule {
	t.Helper()
	r, err := pricing.NewCategoryPromotionRule(category, percent)
	if err != nil {
		t.Fatalf("NewCategoryPromotionRule(%q, %d): %v", category, percent, err)
	}
	return r
}

// ---------- BulkQuantityRule ----------

func TestStep02_Bulk_GivesTenPercentOffTheLine_WhenQuantityIsTenOrMore(t *testing.T) {
	rule := pricing.BulkQuantityRule{}
	if rule.Name() != "BulkQuantity" {
		t.Fatalf("Name() = %q, want %q", rule.Name(), "BulkQuantity")
	}
	if got := rule.DiscountCents(cart(domain.TierStandard, line(1000, 10, "General", 1))); got != 1000 { // 10 x 1000 = 10000 -> 1000
		t.Fatalf("discount = %d, want 1000", got)
	}
}

func TestStep02_Bulk_GivesNothing_WhenQuantityIsBelowTen_ButStillAppliesAtTen(t *testing.T) {
	rule := pricing.BulkQuantityRule{}
	if got := rule.DiscountCents(cart(domain.TierStandard, line(1000, 9, "General", 1))); got != 0 {
		t.Fatalf("quantity 9: discount = %d, want 0", got)
	}
	if got := rule.DiscountCents(cart(domain.TierStandard, line(1000, 10, "General", 1))); got != 1000 { // the boundary
		t.Fatalf("quantity 10: discount = %d, want 1000", got)
	}
}

func TestStep02_Bulk_SumsAllQualifyingLines_AndIgnoresTheRest(t *testing.T) {
	c := cart(domain.TierStandard,
		line(1000, 10, "General", 1), // 10000 -> 1000
		line(500, 20, "General", 2),  // 10000 -> 1000
		line(9999, 2, "General", 3),  // does not qualify
	)
	if got := (pricing.BulkQuantityRule{}).DiscountCents(c); got != 2000 {
		t.Fatalf("discount = %d, want 2000", got)
	}
}

func TestStep02_Bulk_RoundsDown(t *testing.T) {
	// 11 x 15 = 165 cents; 10% of that is 16.5 -> 16
	if got := (pricing.BulkQuantityRule{}).DiscountCents(cart(domain.TierStandard, line(15, 11, "General", 1))); got != 16 {
		t.Fatalf("discount = %d, want 16", got)
	}
}

// ---------- LoyaltyTierRule ----------

func TestStep02_Loyalty_AppliesTheTierPercentageToTheSubtotal(t *testing.T) {
	rule := pricing.LoyaltyTierRule{}
	if rule.Name() != "Loyalty" {
		t.Fatalf("Name() = %q, want %q", rule.Name(), "Loyalty")
	}
	tests := []struct {
		tier domain.CustomerTier
		want int64
	}{
		{domain.TierStandard, 0},
		{domain.TierSilver, 300},
		{domain.TierGold, 500},
	}
	for _, tc := range tests {
		t.Run(string(tc.tier), func(t *testing.T) {
			if got := rule.DiscountCents(cart(tc.tier, line(10000, 1, "General", 1))); got != tc.want {
				t.Fatalf("discount = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStep02_Loyalty_RoundsDown(t *testing.T) {
	// Gold, subtotal 999: 5% = 49.95 -> 49
	if got := (pricing.LoyaltyTierRule{}).DiscountCents(cart(domain.TierGold, line(999, 1, "General", 1))); got != 49 {
		t.Fatalf("discount = %d, want 49", got)
	}
}

// ---------- CategoryPromotionRule ----------

func TestStep02_Category_DiscountsOnlyLinesInThatCategory_IgnoringCase(t *testing.T) {
	c := cart(domain.TierStandard,
		line(5000, 2, "Audio", 1),  // 10000
		line(1000, 3, "Cables", 2), // 3000, not in the promotion
	)
	rule := mustPromo(t, "audio", 20)

	if rule.Name() != "Category:audio" {
		t.Fatalf("Name() = %q, want %q", rule.Name(), "Category:audio")
	}
	if got := rule.DiscountCents(c); got != 2000 {
		t.Fatalf("discount = %d, want 2000", got)
	}
}

func TestStep02_Category_RejectsPercentagesOutsideOneToHundred(t *testing.T) {
	for _, percent := range []int{0, -5, 101} {
		if _, err := pricing.NewCategoryPromotionRule("Audio", percent); !errors.Is(err, pricing.ErrInvalidPercent) {
			t.Errorf("percent %d: error = %v, want pricing.ErrInvalidPercent", percent, err)
		}
	}
	if _, err := pricing.NewCategoryPromotionRule("Audio", 100); err != nil {
		t.Errorf("percent 100 should be accepted, got %v", err)
	}
}

// ---------- Calculator ----------

func names(b pricing.Breakdown) []string {
	var out []string
	for _, d := range b.Discounts {
		out = append(out, d.Rule)
	}
	return out
}

func amounts(b pricing.Breakdown) []int64 {
	var out []int64
	for _, d := range b.Discounts {
		out = append(out, d.AmountCents)
	}
	return out
}

func TestStep02_Calculator_WithNoRules_TotalEqualsSubtotal(t *testing.T) {
	got := pricing.NewCalculator().Calculate(cart(domain.TierGold, line(1000, 3, "General", 1)))

	if got.SubtotalCents != 3000 || got.TotalCents != 3000 || len(got.Discounts) != 0 {
		t.Fatalf("got %+v, want subtotal 3000, total 3000 and no discounts", got)
	}

	// An empty cart is fine too: everything is zero, and rules have nothing to discount.
	empty := pricing.NewCalculator(pricing.LoyaltyTierRule{}).Calculate(cart(domain.TierGold))
	if empty.SubtotalCents != 0 || empty.TotalCents != 0 || len(empty.Discounts) != 0 {
		t.Fatalf("empty cart: got %+v, want all zeros", empty)
	}
}

func TestStep02_Calculator_AppliesRulesInOrder_AndListsEachDiscount(t *testing.T) {
	calc := pricing.NewCalculator(pricing.BulkQuantityRule{}, pricing.LoyaltyTierRule{})

	got := calc.Calculate(cart(domain.TierGold, line(1000, 10, "General", 1))) // subtotal 10000

	if got.SubtotalCents != 10000 {
		t.Fatalf("subtotal = %d, want 10000", got.SubtotalCents)
	}
	if want := []string{"BulkQuantity", "Loyalty"}; !slices.Equal(names(got), want) {
		t.Fatalf("discount names = %v, want %v", names(got), want)
	}
	if want := []int64{1000, 500}; !slices.Equal(amounts(got), want) {
		t.Fatalf("discount amounts = %v, want %v", amounts(got), want)
	}
	if got.TotalCents != 8500 || got.TotalDiscountCents() != 1500 {
		t.Fatalf("total = %d (discount %d), want 8500 (1500)", got.TotalCents, got.TotalDiscountCents())
	}
}

func TestStep02_Calculator_OmitsRulesThatGiveNoDiscount(t *testing.T) {
	calc := pricing.NewCalculator(pricing.BulkQuantityRule{}, pricing.LoyaltyTierRule{})

	got := calc.Calculate(cart(domain.TierStandard, line(1000, 2, "General", 1)))

	if len(got.Discounts) != 0 || got.TotalCents != 2000 {
		t.Fatalf("got %+v, want no discounts and total 2000", got)
	}
}

func TestStep02_Calculator_CapsTotalDiscountAtThirtyPercent_WhenABigRuleComesFirst(t *testing.T) {
	// Subtotal 10000 -> budget 3000. The 50% promotion wants 5000 but only gets 3000; nothing is left for the others.
	calc := pricing.NewCalculator(mustPromo(t, "Audio", 50), pricing.BulkQuantityRule{}, pricing.LoyaltyTierRule{})

	got := calc.Calculate(cart(domain.TierGold, line(1000, 10, "Audio", 1)))

	if want := []string{"Category:Audio"}; !slices.Equal(names(got), want) {
		t.Fatalf("discount names = %v, want %v", names(got), want)
	}
	if want := []int64{3000}; !slices.Equal(amounts(got), want) {
		t.Fatalf("discount amounts = %v, want %v", amounts(got), want)
	}
	if got.TotalCents != 7000 {
		t.Fatalf("total = %d, want 7000", got.TotalCents)
	}
}

func TestStep02_Calculator_RuleOrderMatters_WhenTheCapIsHit(t *testing.T) {
	// Same cart, rules reversed: Bulk 1000 + Loyalty 500 are granted first; the promotion only gets the remaining 1500.
	calc := pricing.NewCalculator(pricing.BulkQuantityRule{}, pricing.LoyaltyTierRule{}, mustPromo(t, "Audio", 50))

	got := calc.Calculate(cart(domain.TierGold, line(1000, 10, "Audio", 1)))

	if want := []int64{1000, 500, 1500}; !slices.Equal(amounts(got), want) {
		t.Fatalf("discount amounts = %v, want %v", amounts(got), want)
	}
	if got.TotalCents != 7000 {
		t.Fatalf("total = %d, want 7000", got.TotalCents)
	}
}

func TestStep02_Calculator_AcceptsBrandNewRules_WithoutModification(t *testing.T) {
	// A rule defined right here: the Calculator needed no change to support it (Open/Closed).
	coupon := pricing.RuleFunc("Coupon", func(pricing.Cart) int64 { return 700 })
	calc := pricing.NewCalculator(coupon)

	got := calc.Calculate(cart(domain.TierStandard, line(1000, 5, "General", 1))) // subtotal 5000, cap 1500

	if want := []string{"Coupon"}; !slices.Equal(names(got), want) {
		t.Fatalf("discount names = %v, want %v", names(got), want)
	}
	if got.TotalCents != 4300 {
		t.Fatalf("total = %d, want 4300", got.TotalCents)
	}
}

func TestStep02_Calculator_TreatsNegativeDiscountsAsZero(t *testing.T) {
	buggy := pricing.RuleFunc("Buggy", func(pricing.Cart) int64 { return -500 })

	got := pricing.NewCalculator(buggy).Calculate(cart(domain.TierStandard, line(1000, 5, "General", 1)))

	if len(got.Discounts) != 0 || got.TotalCents != 5000 {
		t.Fatalf("got %+v, want no discounts and total 5000", got)
	}
}

func TestStep02_Calculator_IsNotAffectedByLaterChangesToTheRulesSlice(t *testing.T) {
	rules := []pricing.Rule{pricing.LoyaltyTierRule{}}
	calc := pricing.NewCalculator(rules...)
	rules[0] = pricing.RuleFunc("Evil", func(pricing.Cart) int64 { return 999999 }) // the caller mutates their slice afterwards

	got := calc.Calculate(cart(domain.TierGold, line(10000, 1, "General", 1)))

	if want := []string{"Loyalty"}; !slices.Equal(names(got), want) {
		t.Fatalf("discount names = %v, want %v (the calculator must keep its own copy of the rules)", names(got), want)
	}
}
