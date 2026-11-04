package fake

import (
	"context"
	"maps"
	"slices"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type Vault struct {
	w   *world.World
	mac string
}

var _ contract.Vault = (*Vault)(nil)

func NewVault(w *world.World, mac string) *Vault { return &Vault{w: w, mac: mac} }

func (v *Vault) kept() map[string][]byte {
	m := mac(v.w, v.mac)
	if m.Vault == nil {
		m.Vault = map[string][]byte{}
	}
	return m.Vault
}

func (v *Vault) Get(_ context.Context, name string) ([]byte, error) {
	secret, ok := v.kept()[name]
	if !ok {
		return nil, accounts.ErrNotKept
	}
	return slices.Clone(secret), nil
}

func (v *Vault) Put(_ context.Context, name string, secret []byte) error {
	v.kept()[name] = slices.Clone(secret)
	return nil
}

func (v *Vault) Forget(_ context.Context, name string) error {
	if _, ok := v.kept()[name]; !ok {
		return accounts.ErrNotKept
	}
	delete(v.kept(), name)
	return nil
}

func (v *Vault) Names(context.Context) ([]string, error) {
	return slices.Sorted(maps.Keys(v.kept())), nil
}
