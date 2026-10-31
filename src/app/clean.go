package app

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	cleancmd "github.com/jterrazz/studio-station/src/cli/clean"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/tart"
)

func cleanCommand(env kit.Env, m machine) *cobra.Command {
	return cleancmd.Command(cleancmd.Deps{Clean: m.clean(), Context: m.context}, env)
}

func (m machine) clean() *clean.Service {
	return clean.New(m.host, catalog.Cleanables, clean.Options{
		Here:      m.kind,
		OldImages: oldImages{images: m.images(), engine: tart.New(m.host.Run)},
		StaleVMs:  staleVMs{machines: m.machines(false)},
	})
}

const gigabyte = 1_000_000_000

type oldImages struct {
	images *images.Service
	engine system.Engine
}

func (o oldImages) List(ctx context.Context) ([]clean.Reclaimable, error) {
	old, err := o.images.Old(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]clean.Reclaimable, 0, len(old))
	for _, r := range old {
		out = append(out, clean.Reclaimable{Name: r.Name + "@" + r.Version, Bytes: int64(r.SizeGB) * gigabyte})
	}
	return out, nil
}

// invariant: the base image a build pulled stays in tart's OCI cache, so every
// removal ends by pruning that cache.
func (o oldImages) Remove(ctx context.Context, name string) error {
	image, version, _ := strings.Cut(name, "@")
	if _, err := o.images.Delete(ctx, image, version); err != nil {
		return err
	}
	return o.engine.PruneCaches(ctx)
}

type staleVMs struct{ machines *machines.Service }

func (s staleVMs) List(ctx context.Context) ([]clean.Reclaimable, error) {
	stale, err := s.machines.Stale(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]clean.Reclaimable, 0, len(stale))
	for _, vm := range stale {
		out = append(out, clean.Reclaimable{Name: vm.Name, Bytes: int64(vm.SizeGB) * gigabyte})
	}
	return out, nil
}

func (s staleVMs) Remove(ctx context.Context, name string) error {
	_, err := s.machines.Delete(ctx, name)
	return err
}
