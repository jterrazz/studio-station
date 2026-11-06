package wire

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/apps"

	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	pagesetup "github.com/jterrazz/studio-station/src/pages/setup"

	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/platform/local"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/setup"
	setupcalls "github.com/jterrazz/studio-station/src/setup/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	setupcmd "github.com/jterrazz/studio-station/src/surface/cli/setup"
)

func setupCommand(env kit.Env, m machine) *cobra.Command {
	return setupcmd.Command(setupcmd.Deps{Here: m.where}, env)
}

func setupService(env kit.Env, m machine) *setupcalls.Service {
	fleet := m.machines(env.Interactive)
	registrar := fleetRegistrar{name: m.stores.name(), machines: fleet}
	installer := engineInstaller{m: m, machines: fleet}
	return setupcalls.NewService(setupMachine{m.host}, setupcalls.ServicePrefs{Name: m.stores.name(), Identity: m.stores.identity(), Hosts: m.stores.hosts(), VMUser: m.stores.vmUser(), ShareList: m.stores.shareList(), Services: m.stores.services(), Stores: m.stores.migrators()}, setupVault{m.vault()}, registrar, installer, setupcalls.ServiceOptions{
		Kind:            m.kind,
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
		Settings:        setupSettings{config: configService(m)},
		Installer:       systemTools{tools: toolsService(env, m)},
		AppStoreCLI:     m.tailnetCLI().AppStoreCLI,
		MovePreferences: m.stores.prefs.MoveRetired,
	})
}

type setupConfigurer struct{ config *settingscalls.Catalogue }

func (c setupConfigurer) Configure(ctx context.Context, item string, interactive bool) error {
	_, err := c.config.Install(ctx, item, nil, interactive)
	return err
}

type setupSettings struct{ config *settingscalls.Catalogue }

func (c setupSettings) List(ctx context.Context) ([]setupcalls.Setting, error) {
	reports, err := c.config.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]setupcalls.Setting, 0, len(reports))
	for _, r := range reports {
		out = append(out, setupcalls.Setting{
			Name: r.Item.Name, Due: r.Applies && r.State == catalog.StateAbsent, ChangedSince: r.Applies && r.State == catalog.StateDegraded,
			AlwaysOn: r.Item.AlwaysOn(), NeedsOwner: r.Item.NeedsOwner(),
		})
	}
	return out, nil
}

func (c setupSettings) Apply(ctx context.Context, name string) error {
	_, err := c.config.Install(ctx, name, nil, false)
	return err
}

type systemTools struct{ tools *appscalls.Service }

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
	outcomes, err := s.tools.Install(ctx, apps.InstallRequest{Names: names, Interactive: true})
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
	machines *machinescalls.Service
}

func (r fleetRegistrar) Here() (string, bool) {
	here, err := r.name.Get()
	return here, err == nil && here != ""
}

func (r fleetRegistrar) Register(name string) error { return r.machines.Init(name) }

type setupRun struct{ *setupcalls.Service }

func (s setupRun) Run(ctx context.Context, name string, interactive bool, ask func() (string, error)) (setup.Report, error) {
	return s.Service.Run(ctx, setupcalls.Request{Name: name, Interactive: interactive, AskName: ask})
}

// value: the name asked in a form at the owner's terminal, from the one
// this machine would take.
type setupAsk struct {
	setup *setupcalls.Service
	ask   kit.Asker
}

func (a setupAsk) Name(ctx context.Context) (string, error) {
	values, err := a.ask.Form(pagesetup.NameForm(a.setup.DefaultName(ctx)))
	return strings.TrimSpace(values["name"]), err
}

// value: the keychain as setup's retirement forgets from it: a Mac without
// its command keeps nothing.
type setupVault struct{ v accountscontract.Vault }

func (s setupVault) Forget(ctx context.Context, name string) error {
	err := s.v.Forget(ctx, name)
	if errors.Is(err, local.ErrNotFound) {
		return accounts.ErrNotKept
	}
	return err
}
