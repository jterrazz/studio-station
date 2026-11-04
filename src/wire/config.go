package wire

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/settings/adapter/dotfile"
	"github.com/jterrazz/studio-station/src/settings/adapter/touch"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	configcmd "github.com/jterrazz/studio-station/src/surface/cli/config"
	configinstall "github.com/jterrazz/studio-station/src/surface/cli/config/install"
	configuninstall "github.com/jterrazz/studio-station/src/surface/cli/config/uninstall"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func configCommand(env kit.Env, m machine) *cobra.Command {
	service := configService(m)
	names := func(keep func(catalog.ConfigItem) bool) func() []string {
		return func() []string {
			var out []string
			for _, it := range service.Placed() {
				if keep(it) {
					out = append(out, it.Name)
				}
			}
			return out
		}
	}
	return configcmd.Command(configcmd.Deps{Names: names(func(catalog.ConfigItem) bool { return true }), Here: m.where}, env,
		configinstall.Command(configinstall.Deps{Names: names(func(catalog.ConfigItem) bool { return true }), Here: m.where}, env),
		configuninstall.Command(configuninstall.Deps{Reversible: names(func(it catalog.ConfigItem) bool { return it.Reversible }), Here: m.where}, env),
	)
}

// value: an item's values asked in a form at the owner's terminal.
type configAsk struct {
	config *config.Service
	ask    kit.Asker
}

func (a configAsk) Values(it catalog.ConfigItem) (map[string]string, error) {
	return a.ask.Form(pageconfig.InputsForm(it.Name, it.Inputs, a.config.Defaults(it.Name)))
}

func configService(m machine) *config.Service {
	// invariant: "dotfiles" is the embedded root, so Sub cannot fail.
	files, _ := fs.Sub(catalog.DotfilesFS, "dotfiles")
	return config.New(m.host, catalog.ConfigItems, config.Prefs{Identity: m.stores.identity(), ShareList: m.stores.shareList()}, config.Options{
		Pool:             m.pool,
		Here:             m.kind,
		DNSProfile:       filepath.Join(m.paths.State, "dns", "quad9-dns.mobileconfig"),
		BackupExclusions: []string{m.paths.Roots.Cache, filepath.Join(m.paths.Data, "vms")},
		Defaults:         configDefaults(m),
		Jobs:             m.jobs(config.ReadBudget),
		Appliers: []settingscontract.Applier{
			touch.New(m.host.FS, m.host.Env.Home()),
			dotfile.New(m.host.FS, m.host.Run, dotfile.Options{Home: m.host.Env.Home(), Ours: files}),
		},
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
