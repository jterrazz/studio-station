package wire

import (
	"context"
	"errors"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"
	"strings"

	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	setupcmd "github.com/jterrazz/studio-station/src/surface/cli/setup"
)

func setupCommand(env kit.Env, m machine) *cobra.Command {
	return setupcmd.Command(setupcmd.Deps{Here: m.where}, env)
}

func setupService(env kit.Env, m machine) *setup.Service {
	fleet := m.machines(env.Interactive)
	registrar := fleetRegistrar{name: m.stores.name(), machines: fleet}
	installer := daemonInstaller{m: m, machines: fleet}
	return setup.New(m.host, setup.Prefs{Name: m.stores.name(), Identity: m.stores.identity(), Hosts: m.stores.hosts(), VMUser: m.stores.vmUser(), ShareList: m.stores.shareList(), Services: m.stores.services(), Stores: m.stores.migrators()}, m.vault(), registrar, installer, setup.Options{
		Here:            m.kind,
		Server:          m.profile.Kind == coremachine.Server,
		PreferencesFile: m.stores.prefs.File(), State: m.paths.State, Bin: m.paths.Bin,
		// hazard: a spec points STATION_EXECUTABLE at a stub so the detached re-exec is
		// observable; unset, the running binary is the one re-executed.
		Executable:      m.or("STATION_EXECUTABLE", m.host.Env.Executable()),
		Detached:        m.vars["STATION_XDG_DETACHED"] == "1",
		Tools:           catalog.Tools,
		Expand:          m.expander(),
		Files:           m.toolFiles(),
		Configurer:      setupConfigurer{config: configService(m)},
		Installer:       systemTools{tools: toolsService(env, m)},
		AppStoreCLI:     m.tailnetCLI().AppStoreCLI,
		MovePreferences: m.stores.prefs.MoveRetired,
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
	name     setupcontract.NamePrefs
	machines *machines.Service
}

func (r fleetRegistrar) Here() (string, bool) {
	here, err := r.name.Get()
	return here, err == nil && here != ""
}

func (r fleetRegistrar) Register(name string) error { return r.machines.Init(name) }

type setupRun struct{ *setup.Service }

func (s setupRun) Run(ctx context.Context, name string, interactive bool, ask func() (string, error)) (setup.Report, error) {
	return s.Service.Run(ctx, setup.Request{Name: name, Interactive: interactive, AskName: ask})
}

// value: the name asked in a form at the owner's terminal, from the one
// this machine would take.
type setupAsk struct {
	setup *setup.Service
	ask   kit.Asker
}

func (a setupAsk) Name(ctx context.Context) (string, error) {
	values, err := a.ask.Form(pagesetup.NameForm(a.setup.DefaultName(ctx)))
	return strings.TrimSpace(values["name"]), err
}
