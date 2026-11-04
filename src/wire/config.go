package wire

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	"github.com/jterrazz/studio-station/src/settings/adapter/dotfile"
	"github.com/jterrazz/studio-station/src/settings/adapter/touch"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	settingscmd "github.com/jterrazz/studio-station/src/surface/cli/settings"
	settingsapply "github.com/jterrazz/studio-station/src/surface/cli/settings/apply"
	settingsdiff "github.com/jterrazz/studio-station/src/surface/cli/settings/diff"
	settingsoff "github.com/jterrazz/studio-station/src/surface/cli/settings/off"
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
	// invariant: "dotfiles" is the embedded root, so Sub cannot fail.
	files, _ := fs.Sub(catalog.DotfilesFS, "dotfiles")
	appliers := []settingscontract.Applier{
		touch.New(m.host.FS, m.host.Env.Home()),
		dotfile.New(m.host.FS, m.host.Run, dotfile.Options{Home: m.host.Env.Home(), Ours: files}),
	}
	if m.world != nil {
		appliers = append(appliers, m.world.procedureAppliers()...)
	}
	return config.New(m.host, catalog.ConfigItems, config.Prefs{Identity: m.stores.identity(), ShareList: m.stores.shareList()}, config.Options{
		Pool:             m.pool,
		Here:             m.kind,
		DNSProfile:       filepath.Join(m.paths.State, "dns", "quad9-dns.mobileconfig"),
		BackupExclusions: []string{m.paths.Roots.Cache, filepath.Join(m.paths.Data, "vms")},
		Defaults:         configDefaults(m),
		Jobs:             m.jobs(config.ReadBudget),
		Appliers:         appliers,
	})
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
