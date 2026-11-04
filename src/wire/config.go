package wire

import (
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	configcmd "github.com/jterrazz/studio-station/src/surface/cli/config"
	configinstall "github.com/jterrazz/studio-station/src/surface/cli/config/install"
	configuninstall "github.com/jterrazz/studio-station/src/surface/cli/config/uninstall"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func configCommand(env kit.Env, m machine) *cobra.Command {
	service := configService(m)
	return configcmd.Command(configcmd.Deps{Config: service, Here: m.where}, env,
		configinstall.Command(configinstall.Deps{Config: service, Here: m.where}, env),
		configuninstall.Command(configuninstall.Deps{Config: service, Here: m.where}, env),
	)
}

func configService(m machine) *config.Service {
	// invariant: "dotfiles" is the embedded root, so Sub cannot fail.
	files, _ := fs.Sub(catalog.DotfilesFS, "dotfiles")
	return config.New(m.host, catalog.ConfigItems, files, m.store, config.Options{
		Pool:     m.pool,
		Here:     m.kind,
		Paths:    m.paths,
		Defaults: configDefaults(m),
		Jobs:     m.jobs(config.ReadBudget),
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
