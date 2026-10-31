package app

import (
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	configcmd "github.com/jterrazz/studio-station/src/cli/config"
	configinstall "github.com/jterrazz/studio-station/src/cli/config/install"
	configuninstall "github.com/jterrazz/studio-station/src/cli/config/uninstall"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/prompt"
)

func configCommand(env kit.Env, m machine) *cobra.Command {
	service := configService(m)
	ask, _ := env.Prompt.(prompt.Asker)
	return configcmd.Command(configcmd.Deps{Config: service, Context: m.context}, env,
		configinstall.Command(configinstall.Deps{Config: service, Ask: ask, Context: m.context}, env),
		configuninstall.Command(configuninstall.Deps{Config: service, Context: m.context}, env),
	)
}

func configService(m machine) *config.Service {
	// invariant: "dotfiles" is the embedded root, so Sub cannot fail.
	files, _ := fs.Sub(catalog.DotfilesFS, "dotfiles")
	return config.New(m.host, catalog.ConfigItems, files, m.store, config.Options{
		Here:     m.kind,
		Paths:    m.paths,
		Defaults: configDefaults(m),
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
