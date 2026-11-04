package fake

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/ports"
)

type Listeners struct{ listening []ports.Listener }

func NewListeners(listening ...ports.Listener) *Listeners { return &Listeners{listening: listening} }

func (l *Listeners) Read(context.Context) ([]ports.Listener, error) {
	return slices.Clone(l.listening), nil
}
