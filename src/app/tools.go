package app

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	toolscmd "github.com/jterrazz/studio-station/src/cli/tools"
	"github.com/jterrazz/studio-station/src/cli/tools/install"
	"github.com/jterrazz/studio-station/src/cli/tools/uninstall"
	"github.com/jterrazz/studio-station/src/cli/tools/upgrade"
	"github.com/jterrazz/studio-station/src/domain/tools"
)

func toolsCommand(env kit.Env, m machine) *cobra.Command {
	service := tools.New(m.host, catalog.Tools, nil, tools.Options{
		Here:        m.kind,
		MiseShims:   filepath.Join(m.or("MISE_DATA_DIR", filepath.Join(m.host.Env.Home(), ".local", "share", "mise")), "shims"),
		Path:        m.vars["PATH"],
		APIURL:      m.or("STATION_GITHUB_API_URL", "https://api.github.com"),
		GitHubToken: m.or("GITHUB_TOKEN", m.vars["GH_TOKEN"]),
	})
	return toolscmd.Command(toolscmd.Deps{Tools: service, Context: m.context}, env,
		toolsinstall.Command(toolsinstall.Deps{Tools: service, Context: m.context}, env),
		toolsuninstall.Command(toolsuninstall.Deps{Tools: service, Context: m.context}, env),
		toolsupgrade.Command(toolsupgrade.Deps{Tools: service, Context: m.context}, env),
	)
}
