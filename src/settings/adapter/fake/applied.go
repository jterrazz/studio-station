package fake

import (
	"context"
	"maps"
	"slices"

	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

type Applied struct{ kept map[string]settings.Applied }

var (
	_ contract.AppliedStore = (*Applied)(nil)
	_ store.Migrator        = (*Applied)(nil)
)

func NewApplied() *Applied { return &Applied{kept: map[string]settings.Applied{}} }

func (a *Applied) Get(_ context.Context, setting string) (settings.Applied, error) {
	got, ok := a.kept[setting]
	if !ok {
		return settings.Applied{}, store.ErrNotFound
	}
	return got, nil
}

func (a *Applied) Put(_ context.Context, applied settings.Applied) error {
	a.kept[applied.Setting] = applied
	return nil
}

func (a *Applied) List(context.Context) ([]settings.Applied, error) {
	out := make([]settings.Applied, 0, len(a.kept))
	for _, name := range slices.Sorted(maps.Keys(a.kept)) {
		out = append(out, a.kept[name])
	}
	return out, nil
}

func (a *Applied) Delete(_ context.Context, setting string) error {
	delete(a.kept, setting)
	return nil
}

func (a *Applied) Migrate(context.Context) error { return nil }
