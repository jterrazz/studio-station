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
	"github.com/jterrazz/studio-station/src/domain/tools"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	appscmd "github.com/jterrazz/studio-station/src/surface/cli/apps"
	appsinstall "github.com/jterrazz/studio-station/src/surface/cli/apps/install"
	appsremove "github.com/jterrazz/studio-station/src/surface/cli/apps/remove"
	appsupdate "github.com/jterrazz/studio-station/src/surface/cli/apps/update"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func appsCommand(env kit.Env, m machine) *cobra.Command {
	names := placedNames(toolsService(env, m))
	return appscmd.Command(appscmd.Deps{Names: names, Here: m.where}, env,
		appsinstall.Command(appsinstall.Deps{Names: names, Here: m.where, Now: m.host.Clock.Now}, env),
		appsupdate.Command(appsupdate.Deps{Here: m.where, Now: m.host.Clock.Now}, env),
		appsremove.Command(appsremove.Deps{Names: names, Here: m.where, Now: m.host.Clock.Now}, env),
	)
}

// invariant: station tools and its verbs are the hidden v2 words of station
// apps for one release (SPEC §4.3); upgrade's --via is update's --method.
func toolsAlias(env kit.Env, m machine) *cobra.Command {
	names := placedNames(toolsService(env, m))
	update := appsupdate.Via(appsupdate.Command(appsupdate.Deps{Here: m.where, Now: m.host.Clock.Now}, env))
	return kit.Alias("tools", appscmd.Command(appscmd.Deps{Names: names, Here: m.where}, env,
		kit.Alias("install", appsinstall.Command(appsinstall.Deps{Names: names, Here: m.where, Now: m.host.Clock.Now}, env), "install"),
		kit.Alias("uninstall", appsremove.Command(appsremove.Deps{Names: names, Here: m.where, Now: m.host.Clock.Now}, env), "remove"),
		kit.Alias("upgrade", update, "update"),
	), "apps")
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
		mas.New(run), npm.New(run, bundle), uv.New(run, bundle),
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
	config *settingscalls.Catalogue
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
		if values, err = c.ask.Form(pagesettings.InputsForm(item, it.Inputs, c.config.Defaults(item))); err != nil {
			return tools.Step{}, err
		}
	}
	change, err := c.config.Install(ctx, item, values, interactive)
	var root *settings.NeedsRootError
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
