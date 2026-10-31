package app

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	toolscmd "github.com/jterrazz/studio-station/src/cli/tools"
	"github.com/jterrazz/studio-station/src/cli/tools/install"
	"github.com/jterrazz/studio-station/src/cli/tools/uninstall"
	"github.com/jterrazz/studio-station/src/cli/tools/upgrade"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/prompt"
)

func toolsCommand(env kit.Env, m machine) *cobra.Command {
	ask, _ := env.Prompt.(prompt.Asker)
	service := tools.New(m.host, catalog.Tools, configurer{config: configService(m), ask: ask}, tools.Options{
		Here:        m.kind,
		MiseShims:   filepath.Join(m.or("MISE_DATA_DIR", filepath.Join(m.host.Env.Home(), ".local", "share", "mise")), "shims"),
		Path:        m.vars["PATH"],
		APIURL:      m.or("STATION_GITHUB_API_URL", "https://api.github.com"),
		GitHubToken: m.or("GITHUB_TOKEN", m.vars["GH_TOKEN"]),
		VMs:         m.machines(env.Interactive),
	})
	return toolscmd.Command(toolscmd.Deps{Tools: service, Context: m.context}, env,
		toolsinstall.Command(toolsinstall.Deps{Tools: service, Context: m.context}, env),
		toolsuninstall.Command(toolsuninstall.Deps{Tools: service, Context: m.context}, env),
		toolsupgrade.Command(toolsupgrade.Deps{Tools: service, Context: m.context}, env),
	)
}

// invariant: a Then item that asks values or a terminal is skipped off one, as a
// row naming the command that applies it later, never as a failure.
type configurer struct {
	config *config.Service
	ask    prompt.Asker
}

func (c configurer) Configure(ctx context.Context, item string, interactive bool) (tools.Step, error) {
	found, err := c.config.Find([]string{item})
	if err != nil {
		return tools.Step{}, err
	}
	it := found[0]
	if c.config.NeedsInput(it) && !interactive {
		return tools.Step{Item: item, Result: catalog.StateAbsent, Detail: "needs a terminal: station config install " + item}, nil
	}
	var values map[string]string
	if len(it.Inputs) > 0 && c.ask != nil {
		if values, err = c.ask.Inputs("Configure "+item, it.Inputs, c.config.Defaults(item)); err != nil {
			return tools.Step{}, err
		}
	}
	change, err := c.config.Install(ctx, item, values, interactive)
	var root *config.NeedsRootError
	if errors.As(err, &root) {
		return tools.Step{Item: item, Result: catalog.StateAbsent, Detail: "needs root: sudo station config install " + item}, nil
	}
	if err != nil {
		return tools.Step{}, err
	}
	detail := ""
	switch n := len(change.Steps); {
	case n == 1:
		detail = change.Steps[0].Label
	case n > 1:
		detail = change.Steps[0].Label + " and " + strconv.Itoa(n-1) + " more"
	}
	return tools.Step{Item: item, Result: catalog.StateOK, Detail: detail}, nil
}
