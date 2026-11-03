package app

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	servicescmd "github.com/jterrazz/studio-station/src/cli/services"
	servicesrelease "github.com/jterrazz/studio-station/src/cli/services/release"
	servicesurl "github.com/jterrazz/studio-station/src/cli/services/url"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
)

func (m machine) services(env kit.Env) *services.Service {
	o := services.Options{Here: m.kind, State: m.paths.State, HereName: m.here(), Remote: fanOutArgv, Exposure: m.exposure}
	d := servicesDaemon{c: daemon.Dial(m.host, m.paths)}
	if m.kind != catalog.Host {
		g := m.guest()
		p := g.Placement()
		o.HereName, o.VMName, o.HostName = p.Host, p.VM, p.Host
		d = servicesDaemon{c: daemon.DialPath(m.host, g.SocketPath())}
	}
	o.Delegate = func(ctx context.Context, host string, words []string) (int, error) {
		return env.On.Remote.Run(ctx, host, env.Interactive, words)
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
	return services.New(m.host, m.store, d, o)
}

func servicesCommand(env kit.Env, m machine) *cobra.Command {
	service := m.services(env)
	return servicescmd.Command(servicescmd.Deps{Services: service, Here: m.where}, env,
		servicesurl.Command(servicesurl.Deps{Services: service, Here: m.where}, env),
		servicesrelease.Command(servicesrelease.Deps{Services: service, Here: m.where}, env),
	)
}
