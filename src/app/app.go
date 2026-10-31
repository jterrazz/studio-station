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
	"github.com/spf13/cobra"

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
	if inTTY && outTTY {
		// hazard: the background query writes an OSC request and reads the reply
		// on stdin, so it runs only when both ends are a terminal that answers.
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

	var root *cobra.Command
	root = cli.NewRoot(kenv, tuiOpener(kenv, func() *cobra.Command { return root }),
		statusCommand(kenv),
		toolsCommand(kenv, here),
		configCommand(kenv),
		cleanCommand(kenv),
		machinesCommand(kenv),
		imagesCommand(kenv),
		secretsCommand(kenv),
		tailnetCommand(kenv),
		runCommand(kenv),
		setupCommand(kenv),
		shellCommand(kenv),
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
	xdg := func(name, fallback string) string {
		if dir := vars[name]; filepath.IsAbs(dir) {
			return filepath.Join(dir, "station")
		}
		return filepath.Join(home, fallback, "station")
	}
	paths := settings.Paths{
		Config: xdg("XDG_CONFIG_HOME", ".config"),
		State:  xdg("XDG_STATE_HOME", filepath.Join(".local", "state")),
		Cache:  xdg("XDG_CACHE_HOME", ".cache"),
		Bin:    filepath.Join(home, ".local", "bin"),
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

func (m machine) here() string {
	current, _ := m.store.Load()
	return current.Here
}
