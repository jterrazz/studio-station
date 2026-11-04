package wire

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/apps/adapter/app"
	"github.com/jterrazz/studio-station/src/apps/adapter/brew"
	"github.com/jterrazz/studio-station/src/apps/adapter/bun"
	"github.com/jterrazz/studio-station/src/apps/adapter/mas"
	"github.com/jterrazz/studio-station/src/apps/adapter/mise"
	"github.com/jterrazz/studio-station/src/apps/adapter/npm"
	"github.com/jterrazz/studio-station/src/apps/adapter/script"
	"github.com/jterrazz/studio-station/src/apps/adapter/shell"
	"github.com/jterrazz/studio-station/src/apps/adapter/skills"
	"github.com/jterrazz/studio-station/src/apps/adapter/uv"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/tools"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	toolscmd "github.com/jterrazz/studio-station/src/surface/cli/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/tools/install"
	"github.com/jterrazz/studio-station/src/surface/cli/tools/uninstall"
	"github.com/jterrazz/studio-station/src/surface/cli/tools/upgrade"
)

func toolsCommand(env kit.Env, m machine) *cobra.Command {
	names := placedNames(toolsService(env, m))
	return toolscmd.Command(toolscmd.Deps{Names: names, Here: m.where}, env,
		toolsinstall.Command(toolsinstall.Deps{Names: names, Here: m.where}, env),
		toolsuninstall.Command(toolsuninstall.Deps{Names: names, Here: m.where}, env),
		toolsupgrade.Command(toolsupgrade.Deps{Here: m.where}, env),
	)
}

func placedNames(s *tools.Service) func() []string {
	return func() []string {
		placed := s.Placed()
		out := make([]string, 0, len(placed))
		for _, t := range placed {
			out = append(out, t.Name)
		}
		return out
	}
}

func toolsService(env kit.Env, m machine) *tools.Service {
	var ask kit.Asker
	if env.Interactive {
		ask = env.Prompt
	}
	registry, bundler := installers(m)
	return tools.New(m.host, catalog.Tools, configurer{config: configService(m), ask: ask}, tools.Options{
		Pool:       m.pool,
		Here:       m.kind,
		Root:       m.systemRoot(),
		Installers: registry,
		Bundler:    bundler,
		VMs:        m.machines(env.Interactive),
		Log:        filepath.Join(m.paths.State, "logs", "apps.log"),
	})
}

func installers(m machine) (contract.Installers, contract.Bundler) {
	run := m.host.Run
	shims := filepath.Join(m.or("MISE_DATA_DIR", filepath.Join(m.host.Env.Home(), ".local", "share", "mise")), "shims")
	// invariant: a bundle runs with mise's shims first once mise is installed,
	// so a formula's post-install finds the runtimes it pins.
	bundle := brew.New(run, func() []string {
		if _, ok := run.LookPath("mise"); ok {
			return []string{"PATH=" + shims + ":" + m.vars["PATH"]}
		}
		return nil
	})
	registry := contract.Installers{}
	for _, i := range []contract.Installer{
		bundle.Formulae(), bundle.Casks(),
		mas.New(run, bundle), npm.New(run, bundle), uv.New(run, bundle),
		bun.New(run), mise.New(run),
		skills.New(skills.Options{
			Run: run, FS: m.host.FS, Net: m.host.Net, Home: m.host.Env.Home(),
			APIURL: m.or("STATION_GITHUB_API_URL", "https://api.github.com"),
			Token:  m.or("GITHUB_TOKEN", m.vars["GH_TOKEN"]),
		}),
		script.New(run), shell.New(run), app.New(),
	} {
		registry[i.Method()] = i
	}
	if m.world != nil {
		// invariant: a world's installers are its fakes, one per method the
		// real registry has, each lined when the real one rides the bundle.
		for method, real := range registry {
			_, lined := real.(contract.Lined)
			registry[method] = m.world.installer(method, lined)
		}
		return registry, m.world.bundler()
	}
	return registry, bundle
}

// invariant: a Then item that asks values or a terminal is skipped off one, as a
// row naming the command that applies it later, never as a failure.
type configurer struct {
	config *config.Service
	ask    kit.Asker
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
		if values, err = c.ask.Form(pageconfig.InputsForm(item, it.Inputs, c.config.Defaults(item))); err != nil {
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
