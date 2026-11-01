package app

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	storagecmd "github.com/jterrazz/studio-station/src/cli/storage"
	storageclean "github.com/jterrazz/studio-station/src/cli/storage/clean"
	storagereset "github.com/jterrazz/studio-station/src/cli/storage/reset"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/prompt"
)

func storageCommand(env kit.Env, m machine) *cobra.Command {
	service := m.storage(env)
	ask, _ := env.Prompt.(prompt.ResetAsker)
	return storagecmd.Command(storagecmd.Deps{Storage: service, Context: m.context}, env,
		storageclean.Command(storageclean.Deps{Storage: service, Context: m.context}, env),
		storagereset.Command(storagereset.Deps{Storage: service, Ask: ask, Context: m.context}, env),
	)
}

func (m machine) storage(env kit.Env) *storage.Service {
	return storage.New(m.host, catalog.Tools, storage.Options{
		Here:      m.kind,
		Expand:    m.expander(),
		Files:     m.toolFiles(),
		Inventory: inventory{tools: toolsService(env, m)},
	})
}

func (m machine) expander() storage.Expander {
	return storage.Expander{Home: m.host.Env.Home(), Roots: m.paths.Roots}
}

// invariant: Go's env file is where `go env GOENV` says: $GOENV when set and
// absolute ("off" writes nothing), else os.UserConfigDir()/go/env, which is
// ~/Library/Application Support on darwin and the XDG config root on linux.
func (m machine) toolFiles() map[catalog.ToolFile]string {
	home := m.host.Env.Home()
	goenv := filepath.Join(home, "Library", "Application Support", "go", "env")
	if m.kind == catalog.Linux {
		goenv = filepath.Join(m.paths.Roots.Config, "go", "env")
	}
	switch v := m.vars["GOENV"]; {
	case v == "off":
		goenv = ""
	case filepath.IsAbs(v):
		goenv = v
	}
	bunfig := filepath.Join(home, ".bunfig.toml")
	if filepath.IsAbs(m.vars["XDG_CONFIG_HOME"]) {
		bunfig = filepath.Join(m.vars["XDG_CONFIG_HOME"], ".bunfig.toml")
	}
	return map[catalog.ToolFile]string{
		catalog.FileGoEnv:  goenv,
		catalog.FileNpmrc:  filepath.Join(home, ".npmrc"),
		catalog.FileBunfig: bunfig,
	}
}

type inventory struct{ tools *tools.Service }

func (i inventory) Installed(ctx context.Context) (map[string]bool, error) {
	reports, err := i.tools.Inventory(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(reports))
	for _, r := range reports {
		out[r.Tool.Name] = r.State == catalog.StateOK || r.State == catalog.StateDegraded
	}
	return out, nil
}
