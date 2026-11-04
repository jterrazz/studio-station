package fake

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/world"
)

type Listeners struct {
	// invariant: what listens on one Mac of the world, so a listen event of
	// the world is read at the next Read.
	w   *world.World
	mac string
}

func ListenersOf(w *world.World, mac string) *Listeners { return &Listeners{w: w, mac: mac} }

func NewListeners(listening ...ports.Listener) *Listeners {
	w := alone()
	w.Mac(here).Listening = listening
	return ListenersOf(w, here)
}

func (l *Listeners) Read(context.Context) ([]ports.Listener, error) {
	l.w.Lock()
	defer l.w.Unlock()
	return slices.Clone(l.w.Mac(l.mac).Listening), nil
}
