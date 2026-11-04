package fake

import (
	"context"
	"slices"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/world"
)

type Identities struct {
	w   *world.World
	mac string
}

var _ contract.Identities = (*Identities)(nil)

func NewIdentities(w *world.World, mac string) *Identities {
	return &Identities{w: w, mac: mac}
}

func (i *Identities) Offers(p catalog.Provider) bool {
	i.w.Lock()
	defer i.w.Unlock()
	_, ok := mac(i.w, i.mac).Identities[p.ID]
	return ok
}

func (i *Identities) Read(_ context.Context, p catalog.Provider) (accounts.Identity, error) {
	i.w.Lock()
	defer i.w.Unlock()
	id, ok := mac(i.w, i.mac).Identities[p.ID]
	if !ok {
		return accounts.Absent(p.ID, accounts.ReasonMissing, "not installed"), nil
	}
	id.Provider = p.ID
	id.Who, id.Scopes, id.Notes = slices.Clone(id.Who), slices.Clone(id.Scopes), slices.Clone(id.Notes)
	return id, nil
}
