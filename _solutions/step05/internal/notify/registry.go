package notify

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnknownChannel   = errors.New("unknown notification channel")
	ErrDuplicateChannel = errors.New("duplicate notification channel name")
)

type Registry struct {
	channels map[string]Channel
}

func NewRegistry(channels ...Channel) (*Registry, error) {
	r := &Registry{channels: make(map[string]Channel, len(channels))}
	for _, c := range channels {
		key := strings.ToLower(c.Name())
		if _, exists := r.channels[key]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateChannel, c.Name())
		}
		r.channels[key] = c
	}
	return r, nil
}

func (r *Registry) Get(name string) (Channel, error) {
	c, ok := r.channels[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownChannel, name)
	}
	return c, nil
}
