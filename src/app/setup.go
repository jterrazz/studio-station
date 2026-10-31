package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	setupcmd "github.com/jterrazz/studio-station/src/cli/setup"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
)

func setupCommand(env kit.Env, m machine) *cobra.Command {
	return setupcmd.Command(setupcmd.Deps{Setup: setupService(env, m), Context: m.context}, env)
}

func setupService(env kit.Env, m machine) *setup.Service {
	registrar := fleetRegistrar{store: m.store, machines: m.machines(env.Interactive)}
	daemon := tailnetDaemon{service: m.tailnet(env.Interactive)}
	return setup.New(m.host, m.store, settings.NewKeychain(m.host.Run).On(m.kind), registrar, daemon, setup.Options{
		Here:  m.kind,
		Paths: m.paths,
		// hazard: a spec points STATION_EXECUTABLE at a stub so the detached re-exec is
		// observable; unset, the running binary is the one re-executed.
		Executable: m.or("STATION_EXECUTABLE", m.host.Env.Executable()),
		Detached:   m.vars["STATION_XDG_DETACHED"] == "1",
	})
}

type fleetRegistrar struct {
	store    settings.Store
	machines *machines.Service
}

func (r fleetRegistrar) Here() (string, bool) {
	current, err := r.store.Load()
	return current.Here, err == nil && current.Here != ""
}

func (r fleetRegistrar) Register(name string) error { return r.machines.Init(name) }

type tailnetDaemon struct{ service *tailnet.Service }

func (d tailnetDaemon) Running(ctx context.Context) (bool, error) {
	return d.service.DaemonRunning(ctx), nil
}

func (d tailnetDaemon) Stop(ctx context.Context) error { return d.service.StopDaemon(ctx) }

func (d tailnetDaemon) Start(ctx context.Context) error { return d.service.StartDaemon(ctx) }
