package pricing

import "strings"

type BulkQuantityRule struct{}

const (
	BulkMinimumQuantity = 10
	BulkPercentOff      = 10
)

func (BulkQuantityRule) Name() string { return "BulkQuantity" }

func (BulkQuantityRule) DiscountCents(cart Cart) int64 {
	var total int64
	for _, line := range cart.Lines {
		if line.Quantity >= BulkMinimumQuantity {
			total += line.TotalCents() * BulkPercentOff / 100 // multiply first, divide last: integer division floors
		}
	}
	return total
}

type LoyaltyTierRule struct{}

func (LoyaltyTierRule) Name() string { return "Loyalty" }

func (LoyaltyTierRule) DiscountCents(cart Cart) int64 {
	percent := int64(0)
	switch cart.Customer.Tier {
	case "Silver":
		percent = 3
	case "Gold":
		percent = 5
	}
	return cart.SubtotalCents() * percent / 100
}

type CategoryPromotionRule struct {
	category   string
	percentOff int
}

func NewCategoryPromotionRule(category string, percentOff int) (*CategoryPromotionRule, error) {
	if percentOff < 1 || percentOff > 100 {
		return nil, ErrInvalidPercent
	}
	return &CategoryPromotionRule{category: category, percentOff: percentOff}, nil
}

func (r *CategoryPromotionRule) Name() string { return "Category:" + r.category }

func (r *CategoryPromotionRule) DiscountCents(cart Cart) int64 {
	var inCategory int64
	for _, line := range cart.Lines {
		if strings.EqualFold(line.Product.Category, r.category) {
			inCategory += line.TotalCents()
		}
	}
	return inCategory * int64(r.percentOff) / 100
}
