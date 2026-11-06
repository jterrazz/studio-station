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
	"github.com/spf13/cobra"

	awakemacos "github.com/jterrazz/studio-station/src/awake/adapter/macos"
	awakecontract "github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	probe "github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/core/fail"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/platform/darwin"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/setup"
	"github.com/jterrazz/studio-station/src/surface/cli"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	shellcmd "github.com/jterrazz/studio-station/src/surface/cli/shell"
	"github.com/jterrazz/studio-station/src/ui/help"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Run(args, env []string, stdin, stdout, stderr *os.File) int {
	env = withToolBins(env, len(args) > 0 && args[0] == "guest")
	frames, frameIn := stdout, stdin
	var childIn io.Reader = stdin
	if serving(args) {
		// invariant: a run that serves frames owns stdin and stdout for them:
		// every other write of it goes to stderr, no child reads its stdin.
		stdout, childIn = stderr, strings.NewReader("")
	}
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	inTTY, outTTY, errTTY := token.IsTerminal(stdin), token.IsTerminal(stdout), token.IsTerminal(stderr)
	session, opened := openWorld(vars, slices.Contains(args, "--page"))
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
	if opened != nil {
		writer.Failure(kit.Translate(opened, ""))
		return int(fail.ExitFailure)
	}
	var world *worldMac
	if session != nil {
		world = session.here()
	}
	here := resolve(local.New(env, childIn, stdout, stderr), darwin.NewDoor(env), vars, world)
	kenv := kit.Env{
		Out:         writer,
		Prompt:      output.Terminal(stdin, stderr),
		Interactive: inTTY && errTTY,
		Screen:      inTTY && outTTY,
		On:          &kit.On{Here: here.here(), Known: here.known},
		Use:         here.uses(),
		Stdin:       frameIn,
		Stdout:      frames,
		Self:        here.profile.Ref,
		Kind:        here.profile.Kind,
		Now:         here.host.Clock.Now,
		Locate:      here.locate,
	}
	router := here.router(kenv)
	defer router.Close()
	kenv.Calls = router
	if session != nil {
		world.engineFrom(func() *process.Daemon { return here.daemon(kenv) })
		session.farFrom(func(w *worldMac) farMac { return here.farMac(kenv, w) })
	}
	kenv.On.Refresh = func(ctx context.Context) error {
		fleet := here.machines(false)
		f, err := fleet.Fleet(ctx, false)
		if err != nil {
			return err
		}
		return fleet.SSHBlocks(ctx, f)
	}

	commands := []*cobra.Command{
		statusCommand(kenv, here),
		activityCommand(kenv, here),
		appsCommand(kenv, here),
		toolsAlias(kenv, here),
		settingsCommand(kenv, here),
		diskCommand(kenv, here),
		storageAlias(kenv, here),
		awakeCommand(kenv, here),
		machinesCommand(kenv, here),
		imagesCommand(kenv, here),
		secretsAlias(kenv, here),
		accountsCommand(kenv, here),
		backgroundCommand(kenv, here),
		retiredDaemonCommand(kenv, here),
		guestCommand(kenv, here),
		engineCommand(kenv, here),
		runCommand(kenv, here),
		setupCommand(kenv, here),
		shellcmd.Command(kenv),
		useCommand(kenv, here),
	}
	commands = append(commands, engineAliases(kenv, here)...)
	commands = append(commands, settingsAliases(kenv, here)...)
	commands = append(commands, awakeAliases(kenv, here)...)
	commands = append(commands, portsCommands(kenv, here)...)
	if session != nil {
		commands = append(commands, session.command())
	}
	root := cli.NewRoot(kenv, tuiOpener(kenv, here, vars["NO_COLOR"] != ""), overviewCommand(kenv, here), commands...)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(Version()),
		fang.WithCommit(Commit()),
		fang.WithErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
			worded := kit.Translate(err, "")
			if wantsJSON(args) {
				writer.FailureJSON(worded)
			}
			writer.Failure(worded)
		}),
		fang.WithColorSchemeFunc(help.FangScheme),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
	here.pool.Close()
	if session != nil {
		if closed := session.close(context.Background()); closed != nil && err == nil {
			err = closed
			writer.Failure(kit.Translate(err, ""))
		}
	}
	if err == nil {
		return int(fail.ExitOK)
	}
	return int(fail.ExitOf(kit.Translate(err, "")))
}

// value: a run that asked --json, before any `--`: its refusal is a
// document too, even one cobra refused before any command ran.
func wantsJSON(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "--json", "--json=true":
			return true
		}
	}
	return false
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

// value: the shell loader puts the global bins of the catalogue on an
// interactive PATH; station appends them to its own, so what it installed
// there answers a non-interactive caller too, and the caller's PATH still
// wins. A VM's guest is started over ssh with the system's PATH, which also
// lacks the shell block's ~/.local/bin and the tool manager's runtimes: it
// appends those first.
func withToolBins(env []string, guest bool) []string {
	home, tools, path, at := "", "", "", -1
	for i, kv := range env {
		switch k, v, _ := strings.Cut(kv, "="); k {
		case "HOME":
			home = v
		case catalog.ToolDataVar:
			tools = v
		case "PATH":
			path, at = v, i
		}
	}
	if home == "" {
		return env
	}
	if !filepath.IsAbs(tools) {
		tools = filepath.Join(home, catalog.ToolData)
	}
	have := strings.Split(path, ":")
	var dirs []string
	if guest {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"), filepath.Join(tools, catalog.ToolShims))
	}
	for _, bin := range catalog.GlobalBins {
		dirs = append(dirs, filepath.Join(home, bin))
	}
	for _, dir := range dirs {
		if !slices.Contains(have, dir) {
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
	// value: this machine's Mac of the spec world, its fakes' slice; nil in
	// every real run.
	world *worldMac
	// value: the host and power door before any world wraps them, from which
	// another Mac of the world is built.
	raw       local.Host
	door      darwin.Door
	far       *late
	pool      *probe.Pool
	host      local.Host
	powerPort awakecontract.Power
	profile   coremachine.Profile
	vars      map[string]string
	kind      catalog.MachineKind
	paths     setup.Paths
	stores    *stores
	where     page.Context
}

func resolve(raw local.Host, door darwin.Door, vars map[string]string, world *worldMac) machine {
	h := raw
	if world != nil {
		h = world.host(raw)
	}
	home := h.Env.Home()
	root := func(name, fallback string) string {
		if dir := vars[name]; filepath.IsAbs(dir) {
			return dir
		}
		return filepath.Join(home, fallback)
	}
	roots := setup.Roots{
		Config: root("XDG_CONFIG_HOME", ".config"),
		Data:   root("XDG_DATA_HOME", filepath.Join(".local", "share")),
		State:  root("XDG_STATE_HOME", filepath.Join(".local", "state")),
		Cache:  root("XDG_CACHE_HOME", ".cache"),
	}
	paths := setup.Paths{
		Config: filepath.Join(roots.Config, "station"),
		Data:   filepath.Join(roots.Data, "station"),
		State:  filepath.Join(roots.State, "station"),
		Cache:  filepath.Join(roots.Cache, "station"),
		Bin:    filepath.Join(home, ".local", "bin"),
		Roots:  roots,
	}
	st := newStores(h.FS, paths)
	if world != nil {
		world.stores(st)
	}
	registered, _ := st.name().Get()
	var budgets probe.Clock = h.Clock
	if world != nil {
		budgets = world.budgets()
	}
	m := machine{
		world:  world,
		raw:    raw,
		door:   door,
		far:    &late{},
		pool:   probe.NewPool(context.Background(), budgets, commandLane, walkLane),
		host:   h,
		vars:   vars,
		paths:  paths,
		stores: st,
	}
	pin, self := vars["STATION_MACHINE_KIND"], selfOf(h)
	m.powerPort = awakemacos.New(door, h.Run, m.systemRoot())
	if world != nil {
		// invariant: a world says what each of its machines is: its Self is
		// read, never the pin every document runs with.
		pin, self, m.powerPort = "", world.self(), world.power()
	}
	name := hereName(registered, h.Env.Hostname())
	m.profile = profileOf(context.Background(), self, pin, name, m.powerPort, func() (string, bool) { h := m.hypervisor(); return h.Name(), h.Installed() })
	m.kind = legacyKind(m.profile)
	m.where = page.Context{Machine: name, Kind: string(m.kind), Registered: registered != ""}
	return m
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
	here, _ := m.stores.name().Get()
	return here
}
