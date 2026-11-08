package pricing

import "shopflow/internal/todo"

// BulkQuantityRule -- STEP 02. Name() = "BulkQuantity".
// Every cart line with Quantity >= 10 gets 10% off THAT LINE's total. Sum the discounts of the qualifying lines;
// round each line's discount down (integer math).
type BulkQuantityRule struct{}

const (
	BulkMinimumQuantity = 10
	BulkPercentOff      = 10
)

func (BulkQuantityRule) Name() string { return "" } // TODO Step 02

func (BulkQuantityRule) DiscountCents(cart Cart) int64 { return 0 } // TODO Step 02

// LoyaltyTierRule -- STEP 02. Name() = "Loyalty".
// A percentage of the whole SUBTOTAL, by customer tier: Standard 0%, Silver 3%, Gold 5%. Round down.
type LoyaltyTierRule struct{}

func (LoyaltyTierRule) Name() string { return "" } // TODO Step 02

func (LoyaltyTierRule) DiscountCents(cart Cart) int64 { return 0 } // TODO Step 02

// CategoryPromotionRule -- STEP 02. A configurable promotion: "X% off everything in category Y".
// Name() = "Category:" + category. Only lines whose product category equals the category (case-insensitive) count.
// NewCategoryPromotionRule must return ErrInvalidPercent when percentOff is outside 1..100.
type CategoryPromotionRule struct {
	category   string
	percentOff int
}

func NewCategoryPromotionRule(category string, percentOff int) (*CategoryPromotionRule, error) {
	return nil, todo.NotImplemented("Step 02: NewCategoryPromotionRule")
}

func (r *CategoryPromotionRule) Name() string { return "" } // TODO Step 02

func (r *CategoryPromotionRule) DiscountCents(cart Cart) int64 { return 0 } // TODO Step 02
