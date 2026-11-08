// Package todo holds the one helper that every unfinished stub uses.
package todo

import (
	"errors"
	"fmt"
)

// ErrNotImplemented marks an unfinished stub. Tests use it to tell "your code returned an error on purpose" from
// "you have not written this yet".
var ErrNotImplemented = errors.New("not implemented yet")

// NotImplemented is what an unfinished stub returns. Stubs return this error (instead of calling panic) so that a
// failing test tells you which step is unfinished, and so one unfinished function cannot crash the whole test binary.
func NotImplemented(what string) error {
	return fmt.Errorf("%w: %s", ErrNotImplemented, what)
}
