package wire

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/disk/adapter/apfs"
	"github.com/jterrazz/studio-station/src/disk/adapter/posix"
	diskcontract "github.com/jterrazz/studio-station/src/disk/contract"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	storagecmd "github.com/jterrazz/studio-station/src/surface/cli/storage"
	storageclean "github.com/jterrazz/studio-station/src/surface/cli/storage/clean"
	storagereset "github.com/jterrazz/studio-station/src/surface/cli/storage/reset"
)

func storageCommand(env kit.Env, m machine) *cobra.Command {
	service := m.storage(env)
	return storagecmd.Command(storagecmd.Deps{Names: service.Names, Here: m.where}, env,
		storageclean.Command(storageclean.Deps{Names: service.Names, Here: m.where}, env),
		storagereset.Command(storagereset.Deps{Names: service.Names, Plan: service.Plan, Here: m.where}, env),
	)
}

func (m machine) storage(env kit.Env) *storage.Service {
	return storage.New(m.host, catalog.Tools, storage.Options{
		Disk:      m.disk(),
		Pool:      m.pool,
		Here:      m.kind,
		Expand:    m.expander(),
		Files:     m.toolFiles(),
		Inventory: inventory{tools: toolsService(env, m)},
	})
}

func (m machine) disk() diskcontract.DiskUsage {
	if m.profile.System.OS == coremachine.Linux {
		return posix.New(m.host.Run)
	}
	return apfs.New(m.host.Run)
}

func (m machine) expander() storage.Expander {
	return storage.Expander{Home: m.host.Env.Home(), Roots: m.paths.Roots, Root: m.systemRoot()}
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
	mise := filepath.Join(m.paths.Roots.Config, "mise", "config.toml")
	if dir := m.vars["MISE_CONFIG_DIR"]; filepath.IsAbs(dir) {
		mise = filepath.Join(dir, "config.toml")
	}
	if file := m.vars["MISE_GLOBAL_CONFIG_FILE"]; filepath.IsAbs(file) {
		mise = file
	}
	return map[catalog.ToolFile]string{
		catalog.FileGoEnv:  goenv,
		catalog.FileNpmrc:  filepath.Join(home, ".npmrc"),
		catalog.FileBunfig: bunfig,
		catalog.FileMise:   mise,
	}
}

type inventory struct{ tools *tools.Service }

func (i inventory) Installed(ctx context.Context, each func(map[string]bool)) {
	for reports := range i.tools.Watch(ctx) {
		out := make(map[string]bool, len(reports))
		for _, r := range reports {
			if !r.Pending {
				out[r.Tool.Name] = r.State == catalog.StateOK || r.State == catalog.StateDegraded
			}
		}
		each(out)
	}
}
