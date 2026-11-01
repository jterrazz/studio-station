package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/prompt"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/host"
)

func Run(args, env []string, stdin, stdout, stderr *os.File) int {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	inTTY, outTTY, errTTY := term.IsTerminal(stdin.Fd()), term.IsTerminal(stdout.Fd()), term.IsTerminal(stderr.Fd())
	dark := true
	if inTTY && outTTY && len(args) > 0 {
		// hazard: the background query writes an OSC request and reads the reply
		// on stdin, so it runs only when both ends are a terminal that answers; bare
		// `station` opens the TUI, which asks itself, and a key typed early must
		// reach it rather than be swallowed as the reply.
		dark = lipgloss.HasDarkBackground(stdin, stdout)
	}
	writer := output.NewWriter(
		output.Stream{W: stdout, IsTerminal: outTTY, Width: widthOf(stdout, outTTY), Dark: dark},
		output.Stream{W: stderr, IsTerminal: errTTY, Width: widthOf(stderr, errTTY), Dark: dark},
		vars,
	)
	here := resolve(host.New(env, stdin, stdout, stderr), vars)
	kenv := kit.Env{
		Out:         writer,
		Prompt:      prompt.New(stdin, stderr),
		Interactive: inTTY && errTTY,
		Screen:      inTTY && outTTY,
		On:          &kit.On{Here: here.here(), Args: args, Remote: remote.New(here.host.Run)},
	}
	kenv.On.Refresh = func(ctx context.Context) error {
		fleet := here.machines(false)
		f, err := fleet.Fleet(ctx, false)
		if err != nil {
			return err
		}
		return fleet.SSHBlocks(ctx, f)
	}

	root := cli.NewRoot(kenv, tuiOpener(kenv, here),
		statusCommand(kenv, here),
		toolsCommand(kenv, here),
		configCommand(kenv, here),
		storageCommand(kenv, here),
		machinesCommand(kenv, here),
		imagesCommand(kenv, here),
		secretsCommand(kenv, here),
		tailnetCommand(kenv, here),
		runCommand(kenv, here),
		setupCommand(kenv, here),
		shellCommand(kenv, here),
	)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(Version()),
		fang.WithCommit(Commit()),
		fang.WithErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
			writer.Failure(kit.Translate(err, ""))
		}),
		fang.WithColorSchemeFunc(fangScheme),
		fang.WithNotifySignal(os.Interrupt),
	)
	if err == nil {
		return int(fail.CodeOK)
	}
	return int(fail.CodeOf(kit.Translate(err, "")))
}

func widthOf(f *os.File, isTerminal bool) int {
	if !isTerminal {
		return 0
	}
	w, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return w
}

type machine struct {
	host    system.Host
	vars    map[string]string
	kind    catalog.MachineKind
	paths   settings.Paths
	store   settings.Store
	context string
}

func resolve(h system.Host, vars map[string]string) machine {
	home := h.Env.Home()
	root := func(name, fallback string) string {
		if dir := vars[name]; filepath.IsAbs(dir) {
			return dir
		}
		return filepath.Join(home, fallback)
	}
	roots := settings.Roots{
		Config: root("XDG_CONFIG_HOME", ".config"),
		Data:   root("XDG_DATA_HOME", filepath.Join(".local", "share")),
		State:  root("XDG_STATE_HOME", filepath.Join(".local", "state")),
		Cache:  root("XDG_CACHE_HOME", ".cache"),
	}
	paths := settings.Paths{
		Config: filepath.Join(roots.Config, "station"),
		Data:   filepath.Join(roots.Data, "station"),
		State:  filepath.Join(roots.State, "station"),
		Cache:  filepath.Join(roots.Cache, "station"),
		Bin:    filepath.Join(home, ".local", "bin"),
		Roots:  roots,
	}
	kind := settings.DetectKind(context.Background(), h, vars["STATION_MACHINE_KIND"])
	store := settings.NewStore(h.FS, paths)
	current, _ := store.Load()
	return machine{
		host:    h,
		vars:    vars,
		kind:    kind,
		paths:   paths,
		store:   store,
		context: present.MachineContext(current.Here, string(kind), current.Here != ""),
	}
}

func (m machine) or(name, fallback string) string {
	if v := m.vars[name]; v != "" {
		return v
	}
	return fallback
}

// value: the root absolute catalogue paths (/Applications, /opt/homebrew, /Library)
// are read under; the spec harness points it into the sandbox.
func (m machine) systemRoot() string {
	if r := m.vars["STATION_SYSTEM_ROOT"]; filepath.IsAbs(r) {
		return r
	}
	return ""
}

func (m machine) here() string {
	current, _ := m.store.Load()
	return current.Here
}
