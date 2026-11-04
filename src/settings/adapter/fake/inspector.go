package fake

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type Inspector struct {
	w   *world.World
	mac string
}

var _ contract.Inspector = (*Inspector)(nil)

func NewInspector(w *world.World, mac string) *Inspector { return &Inspector{w: w, mac: mac} }

func (i *Inspector) Read(_ context.Context, f catalog.Fact) (settings.Fact, error) {
	i.w.Lock()
	defer i.w.Unlock()
	m := i.w.Macs[i.mac]
	if m == nil {
		return settings.Unread(), nil
	}
	fact, ok := m.Facts[f]
	if !ok {
		return settings.Unread(), nil
	}
	return fact, nil
}
