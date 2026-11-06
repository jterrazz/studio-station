package wire

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/jterrazz/studio-station/src/disk"
	diskdocker "github.com/jterrazz/studio-station/src/disk/adapter/docker"
	"github.com/jterrazz/studio-station/src/platform/local"

	"github.com/spf13/cobra"

	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/disk/adapter/apfs"
	"github.com/jterrazz/studio-station/src/disk/adapter/posix"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	diskcontract "github.com/jterrazz/studio-station/src/disk/contract"
	diskcmd "github.com/jterrazz/studio-station/src/surface/cli/disk"
	diskerase "github.com/jterrazz/studio-station/src/surface/cli/disk/erase"
	diskfree "github.com/jterrazz/studio-station/src/surface/cli/disk/free"
	diskmove "github.com/jterrazz/studio-station/src/surface/cli/disk/move"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func diskCommand(env kit.Env, m machine) *cobra.Command {
	names := m.storage(env).Names
	return diskcmd.Command(diskcmd.Deps{Names: names, Here: m.where}, env,
		diskfree.Command(diskfree.Deps{Names: names, Here: m.where}, env),
		diskerase.Command(diskerase.Deps{Names: names, Here: m.where}, env),
		diskmove.Command(diskmove.Deps{Here: m.where, Home: m.host.Env.Home()}, env),
	)
}

// invariant: station storage and its verbs are the hidden v2 words of
// station disk for one release (SPEC §4.3): clean is free, reset is erase.
func storageAlias(env kit.Env, m machine) *cobra.Command {
	names := m.storage(env).Names
	return kit.Alias("storage", diskcmd.Command(diskcmd.Deps{Names: names, Here: m.where}, env,
		kit.Alias("clean", diskfree.Command(diskfree.Deps{Names: names, Here: m.where}, env), "free"),
		kit.Alias("reset", diskerase.Command(diskerase.Deps{Names: names, Here: m.where}, env), "erase"),
	), "disk")
}

func (m machine) storage(env kit.Env) *diskcalls.Service {
	return diskcalls.NewService(diskMachine{m.host}, catalog.Tools, diskcalls.ServiceOptions{
		Disk:       m.disk(),
		Containers: diskdocker.New(m.host.Run),
		Pool:       m.pool,
		Here:       m.kind,
		Expand:     m.expander(),
		Files:      m.toolFiles(),
		Inventory:  inventory{tools: toolsService(env, m)},
		Configurer: setupConfigurer{config: configService(m)},
	})
}

func (m machine) disk() diskcontract.DiskUsage {
	if m.world != nil {
		return m.world.disk()
	}
	if m.profile.System.OS == coremachine.Linux {
		return posix.New(m.host.Run)
	}
	return apfs.New(m.host.Run)
}

func (m machine) expander() disk.Expander {
	return disk.Expander{Home: m.host.Env.Home(), Roots: m.paths.Roots, Root: m.systemRoot()}
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

type inventory struct{ tools *appscalls.Service }

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

type diskMachine struct{ h local.Host }

func (d diskMachine) Now() time.Time { return d.h.Clock.Now() }
func (d diskMachine) Sleep(ctx context.Context, t time.Duration) error {
	return d.h.Clock.Sleep(ctx, t)
}
func (d diskMachine) Glob(pattern string) []string {
	matches, _ := d.h.FS.Glob(pattern)
	return matches
}
func (d diskMachine) Exists(path string) bool {
	_, err := d.h.FS.Stat(path)
	return err == nil
}
func (d diskMachine) Names(dir string) ([]string, error) {
	entries, err := d.h.FS.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
func (d diskMachine) ReadFile(path string) ([]byte, error) { return d.h.FS.ReadFile(path) }
func (d diskMachine) Prune(dir string)                     { _ = d.h.FS.Remove(dir) }
func (d diskMachine) Has(name string) bool {
	_, ok := d.h.Run.LookPath(name)
	return ok
}

func (d diskMachine) Run(ctx context.Context, name string, args, env []string) (diskcalls.Ran, error) {
	ran, err := d.h.Run.Attach(ctx, local.Cmd{Name: name, Args: args, Env: env}, local.ToStderr)
	if errors.Is(err, local.ErrNotFound) {
		return diskcalls.Ran{}, nil
	}
	return diskcalls.Ran{Found: true, Code: ran.ExitCode, LastStderr: ran.LastStderr}, err
}

func (d diskMachine) Keep(file string, data []byte) error {
	if err := d.h.FS.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := d.h.FS.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return d.h.FS.Rename(tmp, file)
}
