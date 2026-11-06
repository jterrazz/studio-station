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
	diskcmd "github.com/jterrazz/studio-station/src/surface/cli/disk"
	diskerase "github.com/jterrazz/studio-station/src/surface/cli/disk/erase"
	diskfree "github.com/jterrazz/studio-station/src/surface/cli/disk/free"
	diskmove "github.com/jterrazz/studio-station/src/surface/cli/disk/move"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func diskCommand(env kit.Env, m machine) *cobra.Command {
	names := m.diskOf(env).Names
	return diskcmd.Command(diskcmd.Deps{Names: names, ThisMachine: m.where}, env,
		diskfree.Command(diskfree.Deps{Names: names, ThisMachine: m.where}, env),
		diskerase.Command(diskerase.Deps{Names: names, ThisMachine: m.where}, env),
		diskmove.Command(diskmove.Deps{ThisMachine: m.where, Home: m.sys.Env.Home()}, env),
	)
}

// invariant: station storage and its verbs are the hidden v2 words of
// station disk for one release (SPEC §4.3): clean is free, reset is erase.
func storageAlias(env kit.Env, m machine) *cobra.Command {
	names := m.diskOf(env).Names
	return kit.Alias("storage", diskcmd.Command(diskcmd.Deps{Names: names, ThisMachine: m.where}, env,
		kit.Alias("clean", diskfree.Command(diskfree.Deps{Names: names, ThisMachine: m.where}, env), "free"),
		kit.Alias("reset", diskerase.Command(diskerase.Deps{Names: names, ThisMachine: m.where}, env), "erase"),
	), "disk")
}

func (m machine) diskOf(env kit.Env) *storage.Service {
	return storage.New(m.sys, catalog.Apps, storage.Options{
		Disk:       m.disk(),
		Pool:       m.pool,
		Here:       m.kind,
		Expand:     m.expander(),
		Files:      m.appFiles(),
		Inventory:  inventory{apps: appsOf(env, m)},
		Configurer: setupSettingsWriter{settings: settingsOf(m)},
	})
}

func (m machine) disk() diskcontract.DiskUsage {
	if m.world != nil {
		return m.world.disk()
	}
	if m.profile.System.OS == coremachine.Linux {
		return posix.New(m.sys.Run)
	}
	return apfs.New(m.sys.Run)
}

func (m machine) expander() storage.Expander {
	return storage.Expander{Home: m.sys.Env.Home(), Roots: m.paths.Roots, Root: m.systemRoot()}
}

// invariant: Go's env file is where `go env GOENV` says: $GOENV when set and
// absolute ("off" writes nothing), else os.UserConfigDir()/go/env, which is
// ~/Library/Application Support on darwin and the XDG config root on linux.
func (m machine) appFiles() map[catalog.AppFile]string {
	home := m.sys.Env.Home()
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
	return map[catalog.AppFile]string{
		catalog.FileGoEnv:  goenv,
		catalog.FileNpmrc:  filepath.Join(home, ".npmrc"),
		catalog.FileBunfig: bunfig,
		catalog.FileMise:   mise,
	}
}

type inventory struct{ apps *tools.Service }

func (i inventory) Installed(ctx context.Context, each func(map[string]bool)) {
	for reports := range i.apps.Watch(ctx) {
		out := make(map[string]bool, len(reports))
		for _, r := range reports {
			if !r.Pending {
				out[r.App.Name] = r.State == catalog.StateOK || r.State == catalog.StateDegraded
			}
		}
		each(out)
	}
}
