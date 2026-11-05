package wire

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/domain/guest"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	portscmd "github.com/jterrazz/studio-station/src/surface/cli/ports"
	portsshare "github.com/jterrazz/studio-station/src/surface/cli/ports/share"
	portsstopsharing "github.com/jterrazz/studio-station/src/surface/cli/ports/stopsharing"
)

func (m machine) services(_ kit.Env) *services.Service {
	o := services.Options{Here: m.kind, State: m.paths.State, HereName: m.here(), Ask: m.askServices, ReleaseOn: m.releaseOn, Exposure: m.exposure,
		Listening: m.listeners("/").Read}
	d := servicesDaemon{c: m.engineAPI()}
	if m.kind != catalog.Host {
		g := m.guest()
		p := m.placement(g)
		o.HereName, o.VMName, o.HostName = p.Host, p.VM, p.Host
		d = servicesDaemon{c: socketAPI(m.host, g.SocketPath())}
	}
	fleet := m.machines(false)
	o.VMHost = func(ctx context.Context, vm string) (string, bool, error) {
		f, err := fleet.Fleet(ctx, false)
		if err != nil {
			return "", false, err
		}
		for _, h := range f.Hosts {
			for _, v := range h.VMs {
				if v.Name == vm {
					return h.Name, h.Here, nil
				}
			}
		}
		var unknown *machines.UnknownMachineError
		if _, err := fleet.Lookup(ctx, vm); err != nil && !errors.As(err, &unknown) {
			return "", false, err
		}
		return "", false, &services.UnknownVMError{Name: vm}
	}
	return services.New(m.host, services.Stores{Services: m.stores.services(), Name: m.stores.name(), Hosts: m.stores.hosts(), Reservations: m.stores.reservations, Declarations: m.stores.declarations}, d, o)
}

// value: what listens on this Mac, read by lsof, the world's in a world.

// value: where this VM runs, as its guest's hello placed it.
func (m machine) placement(g *guest.Service) guest.Placement {
	if m.world != nil {
		if p, inside := m.world.placement(); inside {
			return p
		}
	}
	return g.Placement()
}

// invariant: station ports, and its old words for one release (SPEC 4.3):
// services with url and release, and tailnet, whose state is the first
// line of the ports.
func portsCommands(env kit.Env, m machine) []*cobra.Command {
	list := func(children ...*cobra.Command) *cobra.Command {
		return portscmd.Command(portscmd.Deps{Here: m.where}, env, children...)
	}
	share := func() *cobra.Command { return portsshare.Command(portsshare.Deps{Here: m.where}, env) }
	stop := func() *cobra.Command { return portsstopsharing.Command(portsstopsharing.Deps{Here: m.where}, env) }
	return []*cobra.Command{
		list(share(), stop()),
		kit.Alias("services", list(kit.Alias("url", share(), "share"), kit.Alias("release", stop(), "stop-sharing")), "ports"),
		kit.Alias("tailnet", list(), "ports"),
	}
}
