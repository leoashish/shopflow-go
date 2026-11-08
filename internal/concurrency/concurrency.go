// Package concurrency holds reusable helpers for running work in parallel -- safely.
package concurrency

import "errors"

// ErrInvalidParallelism is returned by Map when maxParallel < 1.
var ErrInvalidParallelism = errors.New("maxParallel must be at least 1")
