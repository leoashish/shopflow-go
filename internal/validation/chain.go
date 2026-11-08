package validation

import (
	"context"

	"shopflow/internal/domain"
	"shopflow/internal/todo"
)

// Chain -- STEP 03. Combines middlewares into ONE Handler.
//
//   - The FIRST middleware you pass is the first to run: Chain(a, b, c) behaves like a(b(c(end))).
//   - "end" is a terminal handler that reports the request as valid: (nil, nil).
//   - Chain() with no middlewares is just that terminal handler.
//
// Hint: build it from the inside out. Start with the terminal handler, then loop over the middlewares BACKWARDS,
// wrapping the handler you have so far:  h = mws[i](h).
func Chain(mws ...Middleware) Handler {
	return func(ctx context.Context, req domain.OrderRequest) ([]Error, error) {
		return nil, todo.NotImplemented("Step 03: Chain")
	}
}
