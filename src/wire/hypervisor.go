package wire

import (
	"context"

	"github.com/jterrazz/studio-station/src/engine"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/adapter/tart"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/platform/local"
)

func (m machine) hypervisor() contract.Hypervisor {
	tool := tart.New(m.host.Run, m.host.FS, m.host.Clock, local.NewKernel(m.vars), m.paths.State)
	if m.world != nil {
		return m.world.hypervisor(tool.Name())
	}
	return tool
}

type engineHypervisor struct{ contract.Hypervisor }

func (h engineHypervisor) Boot(ctx context.Context, vm string, n model.Network) (engine.Run, error) {
	return h.Hypervisor.Boot(ctx, vm, contract.BootOptions{Network: n})
}

func (h engineHypervisor) Adopt(ctx context.Context, vm string) (engine.Run, bool) {
	return h.Hypervisor.Adopt(ctx, vm)
}
