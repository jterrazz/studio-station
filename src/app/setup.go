package app

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	setupcmd "github.com/jterrazz/studio-station/src/cli/setup"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/tools"
)

func setupCommand(env kit.Env, m machine) *cobra.Command {
	return setupcmd.Command(setupcmd.Deps{Setup: setupService(env, m), Context: m.context}, env)
}

func setupService(env kit.Env, m machine) *setup.Service {
	fleet := m.machines(env.Interactive)
	registrar := fleetRegistrar{store: m.store, machines: fleet}
	installer := daemonInstaller{m: m, machines: fleet}
	return setup.New(m.host, m.store, settings.NewKeychain(m.host.Run).On(m.kind), registrar, installer, setup.Options{
		Here:  m.kind,
		Paths: m.paths,
		// hazard: a spec points STATION_EXECUTABLE at a stub so the detached re-exec is
		// observable; unset, the running binary is the one re-executed.
		Executable:  m.or("STATION_EXECUTABLE", m.host.Env.Executable()),
		Detached:    m.vars["STATION_XDG_DETACHED"] == "1",
		Tools:       catalog.Tools,
		Expand:      m.expander(),
		Files:       m.toolFiles(),
		Configurer:  setupConfigurer{config: configService(m)},
		Installer:   systemTools{tools: toolsService(env, m)},
		AppStoreCLI: m.tailnet().AppStoreCLI,
	})
}

type setupConfigurer struct{ config *config.Service }

func (c setupConfigurer) Configure(ctx context.Context, item string, interactive bool) error {
	_, err := c.config.Install(ctx, item, nil, interactive)
	return err
}

type systemTools struct{ tools *tools.Service }

func (s systemTools) names() []string {
	var out []string
	for _, t := range s.tools.Placed() {
		if t.System {
			out = append(out, t.Name)
		}
	}
	return out
}

func (s systemTools) Missing(ctx context.Context) ([]string, error) {
	states, err := s.tools.States(ctx, s.names())
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range s.names() {
		if states[name] != catalog.StateOK {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

func (s systemTools) Superseded(ctx context.Context) []string { return s.tools.Superseded(ctx) }

func (s systemTools) RemoveFormula(ctx context.Context, formula string) error {
	return s.tools.RemoveFormula(ctx, formula)
}

func (s systemTools) Install(ctx context.Context, names []string) error {
	outcomes, err := s.tools.Install(ctx, tools.InstallRequest{Names: names, Interactive: true})
	if err != nil {
		return err
	}
	for _, o := range outcomes {
		if o.Result == catalog.StateBroken {
			return errors.New(o.Name + ": " + o.Detail)
		}
	}
	return nil
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
