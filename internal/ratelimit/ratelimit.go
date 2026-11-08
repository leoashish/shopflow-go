// Package ratelimit limits how fast something may happen.
package ratelimit

import "errors"

// ErrInvalidTokens is returned when a request asks for fewer than 1 token or more than the bucket can ever hold
// (which could never succeed).
var ErrInvalidTokens = errors.New("tokens must be between 1 and the bucket capacity")
