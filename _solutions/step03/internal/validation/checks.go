package validation

import (
	"context"
	"errors"
	"fmt"

	"shopflow/internal/domain"
	"shopflow/internal/money"
)

func CustomerEligibility(customers CustomerGetter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
			c, err := customers.GetByID(ctx, req.CustomerID)
			if errors.Is(err, domain.ErrNotFound) {
				return []Error{{Code: CodeCustomerNotFound, Message: fmt.Sprintf("customer %d does not exist", req.CustomerID)}}, nil
			}
			if err != nil {
				return nil, err // infrastructure trouble is not a validation result
			}
			if !c.Active {
				return []Error{{Code: CodeCustomerInactive, Message: fmt.Sprintf("customer %d is not active", req.CustomerID)}}, nil
			}
			return next(ctx, req)
		}
	}
}

func OrderLines() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
			var errs []Error

			if len(req.Lines) == 0 {
				errs = append(errs, Error{Code: CodeEmptyOrder, Message: "an order needs at least one line"})
			}

			seen := map[int64]int{}
			var duplicates []int64 // keep first-seen order so results are deterministic
			for _, line := range req.Lines {
				if line.Quantity < 1 || line.Quantity > MaxQuantityPerLine {
					errs = append(errs, Error{
						Code:    CodeInvalidQuantity,
						Message: fmt.Sprintf("product %d: quantity must be between 1 and %d (was %d)", line.ProductID, MaxQuantityPerLine, line.Quantity),
					})
				}
				seen[line.ProductID]++
				if seen[line.ProductID] == 2 {
					duplicates = append(duplicates, line.ProductID)
				}
			}
			for _, id := range duplicates {
				errs = append(errs, Error{Code: CodeDuplicateProduct, Message: fmt.Sprintf("product %d appears on more than one line", id)})
			}

			if len(errs) > 0 {
				return errs, nil
			}
			return next(ctx, req)
		}
	}
}

func StockAvailability(products ProductGetter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
			var errs []Error
			for _, line := range req.Lines {
				p, err := products.GetByID(ctx, line.ProductID)
				if errors.Is(err, domain.ErrNotFound) {
					errs = append(errs, Error{Code: CodeProductNotFound, Message: fmt.Sprintf("product %d does not exist", line.ProductID)})
					continue
				}
				if err != nil {
					return nil, err
				}
				if p.Stock < line.Quantity {
					errs = append(errs, Error{
						Code:    CodeInsufficientStock,
						Message: fmt.Sprintf("product %d: requested %d but only %d in stock", line.ProductID, line.Quantity, p.Stock),
					})
				}
			}
			if len(errs) > 0 {
				return errs, nil
			}
			return next(ctx, req)
		}
	}
}

func CreditLimit(customers CustomerGetter, products ProductGetter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
			customer, err := customers.GetByID(ctx, req.CustomerID)
			if errors.Is(err, domain.ErrNotFound) {
				return next(ctx, req) // an earlier check reports a missing customer
			}
			if err != nil {
				return nil, err
			}

			var total int64
			for _, line := range req.Lines {
				p, err := products.GetByID(ctx, line.ProductID)
				if errors.Is(err, domain.ErrNotFound) {
					continue
				}
				if err != nil {
					return nil, err
				}
				total += p.PriceCents * int64(line.Quantity)
			}

			if total > customer.CreditLimitCents {
				return []Error{{
					Code:    CodeCreditLimitExceeded,
					Message: fmt.Sprintf("order total %s exceeds the credit limit of %s", money.Format(total), money.Format(customer.CreditLimitCents)),
				}}, nil
			}
			return next(ctx, req)
		}
	}
}
