package fake

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/core/store"
)

type Shares struct {
	vms    []string
	shares map[string][]accounts.Share
}

var (
	_ contract.ShareStore = (*Shares)(nil)
	_ store.Migrator      = (*Shares)(nil)
)

func NewShares(vms ...string) *Shares {
	return &Shares{vms: vms, shares: map[string][]accounts.Share{}}
}

func (s *Shares) Get(_ context.Context, vm string) ([]accounts.Share, error) {
	return slices.Clone(s.shares[vm]), nil
}

func (s *Shares) Put(_ context.Context, vm string, shares []accounts.Share) error {
	if !slices.Contains(s.vms, vm) {
		return errors.Join(store.ErrNotFound, errors.New("no VM "+vm))
	}
	if len(shares) == 0 {
		delete(s.shares, vm)
		return nil
	}
	s.shares[vm] = slices.Clone(shares)
	return nil
}

func (s *Shares) List(context.Context) (map[string][]accounts.Share, error) {
	return maps.Clone(s.shares), nil
}

func (s *Shares) Delete(_ context.Context, vm string) error {
	delete(s.shares, vm)
	return nil
}

func (s *Shares) Migrate(context.Context) error { return nil }
