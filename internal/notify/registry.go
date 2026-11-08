package notify

import (
	"errors"

	"shopflow/internal/todo"
)

var (
	ErrUnknownChannel   = errors.New("unknown notification channel")
	ErrDuplicateChannel = errors.New("duplicate notification channel name")
)

// Registry -- STEP 05: a registry-style FACTORY. It hands out the channel registered under a name.
//
// Why not a big switch (case "email": return &EmailChannel{} ...)? Because every new channel would force you to edit
// the factory (violates Open/Closed). Here, channels are INJECTED: add a type, register it, done.
//
//   - Names are matched case-insensitively ("EMAIL" finds the channel named "email").
//   - Get(unknown name)  -> an error that wraps ErrUnknownChannel (use fmt.Errorf("...%w", ...) and include the name).
//   - NewRegistry with two channels whose names collide (ignoring case) -> an error that wraps ErrDuplicateChannel.
type Registry struct{}

func NewRegistry(channels ...Channel) (*Registry, error) {
	return nil, todo.NotImplemented("Step 05: NewRegistry")
}

func (r *Registry) Get(name string) (Channel, error) {
	return nil, todo.NotImplemented("Step 05: Registry.Get")
}
