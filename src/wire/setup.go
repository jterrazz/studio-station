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
	return setupcmd.Command(setupcmd.Deps{ThisMachine: m.where}, env)
}

func setupOf(env kit.Env, m machine) *setupcalls.Service {
	fleet := m.machines(env.Interactive)
	registrar := allMachinesRegistrar{name: m.stores.name(), machines: fleet}
	installer := engineInstaller{m: m, machines: fleet}
	return setupcalls.NewService(setupMachine{m.sys}, setupcalls.ServicePrefs{Name: m.stores.name(), Identity: m.stores.identity(), Hosts: m.stores.macs(), VMUser: m.stores.vmUser(), ShareList: m.stores.shareList(), Services: m.stores.ports(), Stores: m.stores.migrators()}, setupVault{m.vault()}, registrar, installer, setupcalls.ServiceOptions{
		Kind:            m.kind,
		Server:          m.profile.Kind == coremachine.Server,
		PreferencesFile: m.stores.prefs.File(), State: m.paths.State, Bin: m.paths.Bin,
		// hazard: a spec points STATION_EXECUTABLE at a stub so the detached re-exec is
		// observable; unset, the running binary is the one re-executed.
		Executable:      m.or("STATION_EXECUTABLE", m.sys.Env.Executable()),
		Detached:        m.vars["STATION_XDG_DETACHED"] == "1",
		Tools:           catalog.Apps,
		Expand:          m.expander(),
		Files:           m.appFiles(),
		Configurer:      setupSettingsWriter{settings: settingsOf(m)},
		Settings:        setupSettings{settings: settingsOf(m)},
		Installer:       systemApps{apps: appsOf(env, m)},
		AppStoreCLI:     m.tailnetCLI().AppStoreCLI,
		MovePreferences: m.stores.prefs.MoveRetired,
	})
}

type setupSettingsWriter struct{ settings *settingscalls.Catalogue }

func (c setupSettingsWriter) Configure(ctx context.Context, item string, interactive bool) error {
	_, err := c.settings.Install(ctx, item, nil, interactive)
	return err
}

type setupSettings struct{ settings *settingscalls.Catalogue }

func (c setupSettings) List(ctx context.Context) ([]setupcalls.Setting, error) {
	reports, err := c.settings.List(ctx)
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
	_, err := c.settings.Install(ctx, name, nil, false)
	return err
}

type systemApps struct{ apps *appscalls.Service }

func (s systemApps) names() []string {
	var out []string
	for _, t := range s.apps.Placed() {
		if t.System {
			out = append(out, t.Name)
		}
	}
	return out
}

func (s systemApps) Missing(ctx context.Context) ([]string, error) {
	states, err := s.apps.States(ctx, s.names())
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

func (s systemApps) Superseded(ctx context.Context) []string { return s.apps.Superseded(ctx) }

func (s systemApps) RemoveFormula(ctx context.Context, formula string) error {
	return s.apps.RemoveFormula(ctx, formula)
}

func (s systemApps) Install(ctx context.Context, names []string) error {
	outcomes, err := s.apps.Install(ctx, apps.InstallRequest{Names: names, Interactive: true})
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

type allMachinesRegistrar struct {
	name     setupcontract.NamePrefs
	machines *machinescalls.Service
}

func (r allMachinesRegistrar) ThisMac() (string, bool) {
	thisMac, err := r.name.Get()
	return thisMac, err == nil && thisMac != ""
}

func (r allMachinesRegistrar) Register(name string) error { return r.machines.Init(name) }

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
