package validation

import (
	"context"

	"shopflow/internal/domain"
)

func Chain(mws ...Middleware) Handler {
	var h Handler = func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
		return nil, nil // end of the chain: nobody objected
	}
	for i := len(mws) - 1; i >= 0; i-- { // inside-out, so the first middleware ends up outermost (= runs first)
		h = mws[i](h)
	}
	return h
}
