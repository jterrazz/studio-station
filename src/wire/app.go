package wire

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"charm.land/fang/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/ui/help"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Run(args, env []string, stdin, stdout, stderr *os.File) int {
	env = withToolBins(env)
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	inTTY, outTTY, errTTY := token.IsTerminal(stdin), token.IsTerminal(stdout), token.IsTerminal(stderr)
	dark := true
	if inTTY && outTTY && len(args) > 0 {
		// hazard: the background query writes an OSC request and reads the reply
		// on stdin, so it runs only when both ends are a terminal that answers; bare
		// `station` opens the TUI, which asks itself, and a key typed early must
		// reach it rather than be swallowed as the reply.
		dark = token.DetectDark(stdin, stdout)
	}
	writer := output.NewWriter(
		output.Stream{W: stdout, IsTerminal: outTTY, Width: widthOf(stdout, outTTY), Height: heightOf(stdout, outTTY), Dark: dark},
		output.Stream{W: stderr, IsTerminal: errTTY, Width: widthOf(stderr, errTTY), Dark: dark},
		vars,
	)
	here := resolve(local.New(env, stdin, stdout, stderr), vars)
	kenv := kit.Env{
		Out:         writer,
		Prompt:      output.Terminal(stdin, stderr),
		Interactive: inTTY && errTTY,
		Screen:      inTTY && outTTY,
		On:          &kit.On{Here: here.here(), Args: args, Remote: remote.New(here.host.Run)},
		Stdin:       stdin,
		Stdout:      stdout,
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
		powerCommand(kenv, here),
		machinesCommand(kenv, here),
		imagesCommand(kenv, here),
		secretsCommand(kenv, here),
		tailnetCommand(kenv, here),
		servicesCommand(kenv, here),
		accountsCommand(kenv, here),
		stopCommand(kenv, here),
		startCommand(kenv, here),
		daemonCommand(kenv, here),
		guestCommand(kenv, here),
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
		fang.WithColorSchemeFunc(help.FangScheme),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
	here.pool.Close()
	if err == nil {
		return int(fail.ExitOK)
	}
	return int(fail.ExitOf(kit.Translate(err, "")))
}

// value: commands 16 at a time (each is its own process; status starts ~70),
// walks one at a time (one walk is eight workers, the measured knee).
const (
	commandLane = 16
	walkLane    = 1
)

func heightOf(f *os.File, isTerminal bool) int {
	if !isTerminal {
		return 0
	}
	_, h := token.Size(f)
	return h
}

func widthOf(f *os.File, isTerminal bool) int {
	if !isTerminal {
		return 0
	}
	w, _ := token.Size(f)
	return w
}

// value: the shell loader puts these bins on an interactive PATH (zsh/zshrc.sh);
// station appends them to its own, so what it installed there answers a
// non-interactive caller too, and the caller's PATH still wins.
func withToolBins(env []string) []string {
	home, path, at := "", "", -1
	for i, kv := range env {
		switch k, v, _ := strings.Cut(kv, "="); k {
		case "HOME":
			home = v
		case "PATH":
			path, at = v, i
		}
	}
	if home == "" {
		return env
	}
	have := strings.Split(path, ":")
	for _, dir := range []string{filepath.Join(".local", "share", "bun", "bin"), filepath.Join(".local", "share", "go", "bin")} {
		if dir = filepath.Join(home, dir); !slices.Contains(have, dir) {
			if path != "" {
				path += ":"
			}
			path += dir
		}
	}
	out := slices.Clone(env)
	if at < 0 {
		return append(out, "PATH="+path)
	}
	out[at] = "PATH=" + path
	return out
}

type machine struct {
	pool  *probe.Pool
	host  system.Host
	vars  map[string]string
	kind  catalog.MachineKind
	paths settings.Paths
	store settings.Store
	where page.Context
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
		pool:  probe.NewPool(context.Background(), h.Clock, commandLane, walkLane),
		host:  h,
		vars:  vars,
		kind:  kind,
		paths: paths,
		store: store,
		where: page.Context{Machine: hereName(current.Here, h.Env.Hostname()), Kind: string(kind), Registered: current.Here != ""},
	}
}

// value: the name the fleet shows this machine by: the one setup gave it, else
// its hostname, as machines names it (one name in every page, §13.5).
func hereName(registered, hostname string) string {
	if registered != "" {
		return registered
	}
	return strings.ToLower(hostname)
}

func (m machine) or(name, fallback string) string {
	if v := m.vars[name]; v != "" {
		return v
	}
	return fallback
}

// value: the root absolute catalogue paths (/Applications, /opt/homebrew,
// /Library, /usr/local) are read under; the spec harness points it into the
// sandbox.
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
