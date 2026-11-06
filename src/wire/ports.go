package wire

import (
	"context"
	"errors"
	"strconv"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/adapter/guest"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	portscmd "github.com/jterrazz/studio-station/src/surface/cli/ports"
	portsdeclare "github.com/jterrazz/studio-station/src/surface/cli/ports/declare"
	portsshare "github.com/jterrazz/studio-station/src/surface/cli/ports/share"
	portsstopsharing "github.com/jterrazz/studio-station/src/surface/cli/ports/stopsharing"
	portsundeclare "github.com/jterrazz/studio-station/src/surface/cli/ports/undeclare"
)

func (m machine) services(_ kit.Env) *portscalls.Service {
	o := portscalls.ServiceOptions{Here: m.kind, State: m.paths.State, HereName: m.here(), Ask: m.askServices, ReleaseOn: m.releaseOn, Exposure: m.exposure,
		Listening: listedOrNone(m.listeners("/").Read), Routes: m.routes(), EngineInstalled: m.engineInstalled}
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
		return "", false, &ports.UnknownVMError{Name: vm}
	}
	return portscalls.NewService(portscalls.Stores{Services: m.stores.services(), Name: m.stores.name(), Hosts: m.stores.hosts(), Reservations: m.stores.reservations, Declarations: m.stores.declarations}, d, o)
}

// value: what listens on this Mac, read by lsof, the world's in a world.

// value: where this VM runs, as its guest's hello placed it.
func (m machine) placement(g *guest.Service) ports.VMPlace {
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
	declare := portsdeclare.Command(portsdeclare.Deps{Here: m.where}, env)
	undeclare := portsundeclare.Command(portsundeclare.Deps{Here: m.where}, env)
	return []*cobra.Command{
		list(share(), stop(), declare, undeclare),
		kit.Alias("services", list(kit.Alias("url", share(), "share"), kit.Alias("release", stop(), "stop-sharing")), "ports"),
		kit.Alias("tailnet", list(), "ports"),
	}
}

// invariant: what a setting declares public goes through ports' own calls,
// on this Mac: the declarations are the ports family's to write.
type settingsPublic struct{ calls call.Router }

func (p settingsPublic) Declare(ctx context.Context, d catalog.Declares, by string) error {
	in := portscalls.DeclareIn{Port: strconv.Itoa(d.Port), Via: d.Via, By: by}
	_, err := call.Invoke(ctx, p.calls, coremachine.Ref{}, portscalls.Declare, in, nil)
	return err
}

func (p settingsPublic) Undeclare(ctx context.Context, d catalog.Declares) error {
	in := portscalls.UndeclareIn{Port: strconv.Itoa(d.Port), Confirm: confirm.Yes}
	_, err := call.Invoke(ctx, p.calls, coremachine.Ref{}, portscalls.Undeclare, in, nil)
	if f, ok := errors.AsType[*fail.Error](err); ok && f.Code == ports.CodeNotDeclared {
		return nil
	}
	return err
}

// invariant: a Mac with nothing to list its listeners with lists none.
func listedOrNone(read func(ctx context.Context) ([]ports.Listener, error)) func(ctx context.Context) ([]ports.Listener, error) {
	return func(ctx context.Context) ([]ports.Listener, error) {
		listed, err := read(ctx)
		if errors.Is(err, local.ErrNotFound) {
			return nil, nil
		}
		return listed, err
	}
}

func (m machine) engineInstalled() bool {
	_, err := m.host.FS.Stat(catalog.EnginePlist(m.host.Env.Home()))
	return err == nil
}
