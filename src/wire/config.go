package wire

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/domain/config"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	settingsdefaults "github.com/jterrazz/studio-station/src/settings/adapter/defaults"
	"github.com/jterrazz/studio-station/src/settings/adapter/dotfile"
	"github.com/jterrazz/studio-station/src/settings/adapter/git"
	"github.com/jterrazz/studio-station/src/settings/adapter/macos"
	"github.com/jterrazz/studio-station/src/settings/adapter/touch"
	"github.com/jterrazz/studio-station/src/settings/adapter/vault"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	settingscmd "github.com/jterrazz/studio-station/src/surface/cli/settings"
	settingsapply "github.com/jterrazz/studio-station/src/surface/cli/settings/apply"
	settingsdiff "github.com/jterrazz/studio-station/src/surface/cli/settings/diff"
	settingsoff "github.com/jterrazz/studio-station/src/surface/cli/settings/off"
	"github.com/jterrazz/studio-station/src/system"
)

func settingsCommand(env kit.Env, m machine) *cobra.Command {
	all := func() []string {
		out := make([]string, 0, len(catalog.ConfigItems))
		for _, it := range catalog.ConfigItems {
			out = append(out, it.Name)
		}
		return out
	}
	children := func() []*cobra.Command {
		return []*cobra.Command{
			settingsdiff.Command(settingsdiff.Deps{Names: all, Here: m.where}, env),
			settingsapply.Command(settingsapply.Deps{Names: all, Here: m.where}, env),
			settingsoff.Command(settingsoff.Deps{Names: all, Here: m.where}, env),
		}
	}
	return settingscmd.Command(settingscmd.Deps{Names: all, Here: m.where, Now: m.host.Clock.Now}, env, children()...)
}

// value: station config, config install and config uninstall, the words v2
// gave settings, settings apply and settings off.
func settingsAliases(env kit.Env, m machine) []*cobra.Command {
	all := func() []string { return nil }
	return []*cobra.Command{kit.Alias("config", settingscmd.Command(settingscmd.Deps{Names: all, Here: m.where, Now: m.host.Clock.Now}, env,
		kit.Alias("install", settingsapply.Command(settingsapply.Deps{Names: all, Here: m.where}, env), "apply"),
		kit.Alias("uninstall", settingsoff.Command(settingsoff.Deps{Names: all, Here: m.where}, env), "off"),
	), "settings")}
}

// value: an item's values asked in a form at the owner's terminal.
type configAsk struct {
	config *config.Service
	ask    kit.Asker
}

func (a configAsk) Values(it catalog.ConfigItem) (map[string]string, error) {
	return a.ask.Form(pagesettings.InputsForm(it.Name, it.Inputs, a.config.Defaults(it.Name)))
}

func configService(m machine) *config.Service {
	return config.New(m.host, catalog.ConfigItems, config.Options{
		Here:     m.kind,
		Defaults: configDefaults(m),
		Appliers: settingAppliers(m),
	})
}

// value: the Applier of every effect of the catalogue on this machine; in
// a world, every procedure is a fake over the Mac's slice: no world runs a
// procedure of the real machine.
func settingAppliers(m machine) []settingscontract.Applier {
	// invariant: "dotfiles" is the embedded root, so Sub cannot fail.
	files, _ := fs.Sub(catalog.DotfilesFS, "dotfiles")
	appliers := []settingscontract.Applier{
		touch.New(m.host.FS, m.host.Env.Home()),
		dotfile.New(m.host.FS, m.host.Run, dotfile.Options{Home: m.host.Env.Home(), Ours: files}),
	}
	if m.world != nil {
		return append(appliers, m.world.procedureAppliers()...)
	}
	mac := macos.NewMac(m.host.Run, m.host.FS, macos.MacOptions{
		Home:             m.host.Env.Home(),
		DNSProfile:       filepath.Join(m.paths.State, "dns", "quad9-dns.mobileconfig"),
		BackupExclusions: []string{m.paths.Roots.Cache, filepath.Join(m.paths.Data, "vms")},
		Jobs:             m.jobs(config.ReadBudget),
		Identity:         m.stores.identity(),
	})
	appliers = append(appliers, mac.Appliers()...)
	appliers = append(appliers, settingsdefaults.NewDock(m.host.Run)...)
	return append(appliers,
		vault.NewClaudeToken(m.host.Run, m.stores.shareList()),
		git.NewSigning(m.host.Run, m.stores.identity()),
		git.NewLogin(m.host.Run, m.sharedRead()),
	)
}

// value: a read run once per run of the shared pool, whoever asks it.
func (m machine) sharedRead() git.Read {
	if m.pool == nil {
		return nil
	}
	return func(ctx context.Context, c system.Cmd) (system.Output, error) {
		return lanes.Shared(ctx, m.pool, lanes.Command(c.Name, c.Args), func(ctx context.Context) (system.Output, error) { return m.host.Run.Output(ctx, c) })
	}
}

func configDefaults(m machine) map[string]map[string]string {
	defaults := map[string]map[string]string{}
	for _, it := range catalog.ConfigItems {
		for _, in := range it.Inputs {
			if v := strings.TrimSpace(m.vars[in.DefaultEnv]); in.DefaultEnv != "" && v != "" {
				if defaults[it.Name] == nil {
					defaults[it.Name] = map[string]string{}
				}
				defaults[it.Name][in.Name] = v
			}
		}
	}
	return defaults
}
