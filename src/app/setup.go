package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	setupcmd "github.com/jterrazz/studio-station/src/cli/setup"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
)

func setupCommand(env kit.Env, m machine) *cobra.Command {
	service := setup.New(m.host, m.store, settings.NewKeychain(m.host.Run).On(m.kind), settingsRegistrar{store: m.store, user: m.host.Env.User()}, noDaemon{}, setup.Options{
		Here:  m.kind,
		Paths: m.paths,
		// hazard: a spec points STATION_EXECUTABLE at a stub so the detached re-exec is
		// observable; unset, the running binary is the one re-executed.
		Executable: m.or("STATION_EXECUTABLE", m.host.Env.Executable()),
		Detached:   m.vars["STATION_XDG_DETACHED"] == "1",
	})
	return setupcmd.Command(setupcmd.Deps{Setup: service, Context: m.context}, env)
}

// hazard: stands in for the fleet registry until L-INT adapts machines.Service.Init
// over the same two methods.
type settingsRegistrar struct {
	store settings.Store
	user  string
}

func (r settingsRegistrar) Here() (string, bool) {
	current, err := r.store.Load()
	return current.Here, err == nil && current.Here != ""
}

func (r settingsRegistrar) Register(name string) error {
	current, err := r.store.Load()
	if err != nil {
		return err
	}
	current.Here = name
	if current.Hosts == nil {
		current.Hosts = map[string]settings.Host{}
	}
	if _, ok := current.Hosts[name]; !ok {
		current.Hosts[name] = settings.Host{}
	}
	if current.VMUser == "" {
		current.VMUser = r.user
	}
	return r.store.Save(current)
}

// hazard: stands in for the tailnet's daemon control until L-INT adapts it: the
// tailnet domain has no daemon of its own yet, so there is nothing to ask or restart.
type noDaemon struct{}

func (noDaemon) Running(context.Context) (bool, error) { return false, nil }
func (noDaemon) Stop(context.Context) error            { return nil }
func (noDaemon) Start(context.Context) error           { return nil }
