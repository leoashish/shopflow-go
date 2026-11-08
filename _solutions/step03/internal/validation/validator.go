package validation

func NewOrderValidator(customers CustomerGetter, products ProductGetter) Handler {
	return Chain(
		CustomerEligibility(customers),
		OrderLines(),
		StockAvailability(products),
		CreditLimit(customers, products),
	)
}
