//go:build world

package wire

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	accountsfake "github.com/jterrazz/studio-station/src/accounts/adapter/fake"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	activityfake "github.com/jterrazz/studio-station/src/activity/adapter/fake"
	activitycontract "github.com/jterrazz/studio-station/src/activity/contract"
	appsfake "github.com/jterrazz/studio-station/src/apps/adapter/fake"
	appscontract "github.com/jterrazz/studio-station/src/apps/contract"
	awakefake "github.com/jterrazz/studio-station/src/awake/adapter/fake"
	awakecontract "github.com/jterrazz/studio-station/src/awake/contract"
	backgroundfake "github.com/jterrazz/studio-station/src/background/adapter/fake"
	backgroundcontract "github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	corefake "github.com/jterrazz/studio-station/src/core/adapter/fake"
	"github.com/jterrazz/studio-station/src/core/call"
	corecontract "github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/core/fail"
	diskfake "github.com/jterrazz/studio-station/src/disk/adapter/fake"
	diskcontract "github.com/jterrazz/studio-station/src/disk/contract"
	"github.com/jterrazz/studio-station/src/engine"
	enginefake "github.com/jterrazz/studio-station/src/engine/adapter/fake"
	imagesfake "github.com/jterrazz/studio-station/src/images/adapter/fake"
	machinesfake "github.com/jterrazz/studio-station/src/machines/adapter/fake"
	"github.com/jterrazz/studio-station/src/machines/contract"
	portsfake "github.com/jterrazz/studio-station/src/ports/adapter/fake"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	remotefake "github.com/jterrazz/studio-station/src/remote/adapter/fake"
	remotecontract "github.com/jterrazz/studio-station/src/remote/contract"
	settingsfake "github.com/jterrazz/studio-station/src/settings/adapter/fake"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/world"
)

// value: how many quiet ticks end a stepped engine's settling, and the most
// ticks it may take: a world never waits on a clock of its own.
const (
	quietTicks = 3
	mostTicks  = 120
)

// invariant: the spec build's world: read from $STATION_WORLD at start (seeded
// from $STATION_WORLD_SEED the first time, its Macs given the hypervisor
// $STATION_WORLD_HYPERVISOR names when set), every fake over it, each of its
// Macs built in this process as its own binary would build it there, and
// the world written back once the command and every engine are done with it.
type worldRun struct {
	path string
	w    *world.World
	macs map[string]*worldMac
	// value: builds another Mac of the world: its registry, its engine.
	far   func(w *worldMac) farMac
	moved *moves
}

// invariant: one Mac of the world as this process sees it: its fakes over
// its slice, its engine stepped in this process while the world says it runs.
type worldMac struct {
	run  *worldRun
	name string

	start    func() *engine.Daemon
	starting bool
	engine   *engine.Daemon
	tick     int
	served   *remotefake.Dialer
}

func openWorld(vars map[string]string) (*worldRun, error) {
	path := vars["STATION_WORLD"]
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && vars["STATION_WORLD_SEED"] != "" {
		data, err = os.ReadFile(vars["STATION_WORLD_SEED"])
	}
	if err != nil {
		return nil, err
	}
	w, err := world.Decode(data)
	if err != nil {
		return nil, err
	}
	w.UseHypervisor(vars["STATION_WORLD_HYPERVISOR"])
	return &worldRun{path: path, w: w, macs: map[string]*worldMac{}}, nil
}

func (r *worldRun) mac(name string) *worldMac {
	if m, ok := r.macs[name]; ok {
		return m
	}
	m := &worldMac{run: r, name: name}
	r.macs[name] = m
	return m
}

func (r *worldRun) here() *worldMac { return r.mac(r.w.Here) }

func (r *worldRun) farFrom(far func(w *worldMac) farMac) { r.far = far }

func (r *worldRun) names() []string {
	r.w.Lock()
	defer r.w.Unlock()
	return slices.Sorted(maps.Keys(r.w.Macs))
}

func (m *worldMac) slice() world.Mac {
	m.run.w.Lock()
	defer m.run.w.Unlock()
	return *m.run.w.Mac(m.name)
}

func (m *worldMac) runs() bool { mac := m.slice(); return mac.EngineRuns() }

func (m *worldMac) host(h system.Host) system.Host {
	h.Run = rootless{Runner: h.Run, w: m.run.w}
	h.FS = worldFS{FS: h.FS, m: m, plist: catalog.EnginePlist(h.Env.Home())}
	h.Env = worldEnv{Env: h.Env, here: m.name, user: m.run.w.User}
	h.Clock = m.run.clock()
	h.Sockets = worldSockets{Sockets: h.Sockets, m: m}
	return h
}

func (m *worldMac) hypervisor(name string) contract.Hypervisor {
	return machinesfake.HypervisorOf(m.run.w, m.name, name)
}

// invariant: station's own state in a world is its Mac's slice: every store
// is the fake over it, never a file of the run's home.
func (m *worldMac) stores(st *stores) {
	m.run.w.Lock()
	slice := m.run.w.Mac(m.name)
	if slice.Preferences == nil {
		slice.Preferences = map[string]json.RawMessage{}
	}
	if slice.Files == nil {
		slice.Files = map[string]string{}
	}
	st.prefs = corefake.PreferencesOver(slice.Preferences, m.name, m.run.w).Over(slice.Files)
	m.run.w.Unlock()
	w := m.run.w
	st.machines = machinesfake.StoreOf(w, m.name)
	st.images = imagesfake.StoreOf(w, m.name)
	st.shares = accountsfake.SharesOf(w, m.name)
	st.reservations = portsfake.ReservationsOf(w, m.name)
	st.declarations = portsfake.DeclarationsOf(w, m.name)
	st.applied = settingsfake.AppliedOf(w, m.name)
}

func (m *worldMac) self() corecontract.Self    { return corefake.SelfOf(m.run.w, m.name) }
func (m *worldMac) power() awakecontract.Power { return awakefake.PowerOf(m.run.w, m.name) }
func (m *worldMac) guests() contract.GuestDialer {
	return machinesfake.GuestsOf(m.run.w).Serving(engine.GuestSession)
}
func (m *worldMac) sshConfig() contract.SSHConfig  { return machinesfake.SSHConfigOf(m.run.w, m.name) }
func (m *worldMac) unlocker() contract.Unlocker    { return machinesfake.MacsOf(m.run.w) }
func (m *worldMac) jobs() backgroundcontract.Jobs  { return backgroundfake.New(m.run.w, m.name) }
func (m *worldMac) meter() activitycontract.Meter  { return activityfake.MeterOf(m.run.w, m.name) }
func (m *worldMac) disk() diskcontract.DiskUsage   { return diskfake.UsageOf(m.run.w, m.name) }
func (m *worldMac) tailnet() portscontract.Tailnet { return portsfake.TailnetOf(m.run.w, m.name) }
func (m *worldMac) bundler() appscontract.Bundler  { return appsfake.BundlerOf(m.run.w, m.name) }
func (m *worldMac) vault() accountscontract.Vault  { return accountsfake.NewVault(m.run.w, m.name) }
func (m *worldMac) listeners() portscontract.Listeners {
	return portsfake.ListenersOf(m.run.w, m.name)
}

func (m *worldMac) inspector() settingscontract.Inspector {
	return settingsfake.NewInspector(m.run.w, m.name)
}

func (m *worldMac) installer(method catalog.Method, lined bool) appscontract.Installer {
	return appsfake.InstallerOf(m.run.w, m.name, method, lined)
}

func (m *worldMac) identities() accountscontract.Identities {
	return accountsfake.NewIdentities(m.run.w, m.name)
}

func (m *worldMac) secretSource() accountscontract.SecretSource {
	return accountsfake.NewSecretSource(m.run.w, m.name)
}

func (m *worldMac) notifier() engine.Notifier {
	return lockedNotifier{w: m.run.w, n: enginefake.NewNotifier(m.run.w)}
}

func (m *worldMac) engineFrom(start func() *engine.Daemon) { m.start = start }

func (m *worldMac) engineVersion() string {
	if e := m.slice().Engine; e != nil {
		return e.Version
	}
	return ""
}

func (m *worldMac) stationVersion() string { return m.slice().Station }

func (m *worldMac) engineStarted() time.Time {
	if e := m.slice().Engine; e != nil {
		return e.StartedAt
	}
	return time.Time{}
}

// invariant: a stop the owner asked is kept on the Mac's engine of the
// world, as the real one keeps it beside its socket.
func (m *worldMac) stopMarks() engine.StopMarks { return worldMarks{m: m} }

type worldMarks struct{ m *worldMac }

func (w worldMarks) Read() (engine.StopMark, bool) {
	e := w.m.slice().Engine
	if e == nil || e.StoppedAt.IsZero() {
		return engine.StopMark{}, false
	}
	return engine.StopMark{At: e.StoppedAt, By: e.StoppedBy}, true
}

func (w worldMarks) Write(mark engine.StopMark) error {
	w.m.run.w.Lock()
	defer w.m.run.w.Unlock()
	if e := w.m.run.w.Mac(w.m.name).Engine; e != nil {
		e.StoppedAt, e.StoppedBy = mark.At, mark.By
	}
	return nil
}

// invariant: the engine's job definition is the job manager's file; in a
// world it is there while the Mac's engine is set up, whatever the run's
// home holds.
type worldFS struct {
	system.FS
	m     *worldMac
	plist string
}

func (f worldFS) Stat(p string) (fs.FileInfo, error) {
	if p != f.plist {
		return f.FS.Stat(p)
	}
	f.m.run.w.Lock()
	mac := f.m.run.w.Mac(f.m.name)
	set := mac.Engine != nil || mac.Jobs[catalog.EngineLabel] != nil
	f.m.run.w.Unlock()
	if !set {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return plistInfo{name: filepath.Base(p)}, nil
}

type plistInfo struct{ name string }

func (i plistInfo) Name() string     { return i.name }
func (plistInfo) Size() int64        { return 0 }
func (plistInfo) Mode() fs.FileMode  { return 0o644 }
func (plistInfo) ModTime() time.Time { return time.Time{} }
func (plistInfo) IsDir() bool        { return false }
func (plistInfo) Sys() any           { return nil }

func (m *worldMac) macs() remotecontract.EndpointDialer { return worldDialer{r: m.run} }

// invariant: a Mac's engine runs in this process while the world says it
// runs, and ends once it says otherwise: begun at the first need of it,
// then stepped over the world's clock until it is quiet, so a story that
// waits on the engine is deterministic.
func (m *worldMac) up(ctx context.Context) (*engine.Daemon, bool) {
	if m.engine != nil {
		if m.runs() {
			return m.engine, true
		}
		m.down(false)
		return nil, false
	}
	if m.start == nil && m.name != m.run.w.Here && m.run.far != nil && m.runs() {
		m.serve()
	}
	if m.start == nil || m.starting || !m.runs() {
		return nil, false
	}
	m.starting = true
	defer func() { m.starting = false }()
	d := m.start()
	d.Remember(m.slice().Engine.Memory)
	if err := d.Begin(context.WithValue(ctx, waits{}, true), func(string) {}); err != nil {
		return nil, false
	}
	m.engine = d
	m.settle(ctx)
	return d, true
}

func (m *worldMac) settle(ctx context.Context) {
	quiet := 0
	for n := 0; n < mostTicks && quiet < quietTicks; n++ {
		m.tick++
		_ = m.run.clock().Sleep(ctx, time.Second)
		if m.engine.Step(ctx, m.tick) {
			quiet = 0
		} else {
			quiet++
		}
	}
}

// invariant: an engine that ends keeps what it knew in the world, unless its
// process ends with it (a reboot, a crash): its successor knows nothing.
func (m *worldMac) down(forget bool) {
	if m.engine == nil {
		return
	}
	m.engine.Stop()
	memory := m.engine.Memory()
	m.engine = nil
	if forget {
		memory = nil
	}
	m.run.w.Lock()
	defer m.run.w.Unlock()
	if e := m.run.w.Mac(m.name).Engine; e != nil {
		e.Memory = memory
	}
}

func (m *worldMac) serve() *remotefake.Dialer {
	if m.served == nil {
		far := m.run.far(m)
		m.served = remotefake.New()
		m.served.Serve(m.name, remotefake.Mac{Registry: far.registry, Profile: far.profile, Version: cmp.Or(m.stationVersion(), Version()), Word: func(err error) *fail.Error { return kit.Translate(err, "") }})
		m.start = far.start
	}
	return m.served
}

// invariant: another Mac of the world answers while it is on the tailnet,
// unlocked and set up, its engine stopped or not, as `station engine attach`
// answers over ssh; it is then reached as over ssh, every value through the
// wire form.
type worldDialer struct{ r *worldRun }

func (d worldDialer) Dial(ctx context.Context, mac string) (remotecontract.Endpoint, error) {
	d.r.w.Lock()
	slice, ok := d.r.w.Macs[mac]
	reach := ok && !slice.Offline && !slice.Locked && slice.Engine != nil
	d.r.w.Unlock()
	if !reach || d.r.far == nil {
		return nil, fail.New(fail.Unreachable, call.Unreachable, mac, nil)
	}
	return d.r.mac(mac).serve().Dial(ctx, mac)
}

func (r *worldRun) command() *cobra.Command {
	return &cobra.Command{
		Use:    "world <event> [args]",
		Short:  "Apply an event to the spec world: a reboot, a crash, time passing",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			ev, err := world.Parse(args)
			if err != nil {
				return err
			}
			if ev.Name == "edit" && len(ev.Args) == 2 {
				text, readErr := io.ReadAll(c.InOrStdin())
				if readErr != nil {
					return readErr
				}
				ev.Args = append(ev.Args, string(text))
			}
			for _, name := range r.names() {
				r.mac(name).up(c.Context())
			}
			r.w.Lock()
			err = r.w.Apply(ev)
			r.w.Unlock()
			if err != nil {
				return err
			}
			for _, name := range r.names() {
				if m := r.mac(name); restarts(ev, name) || !m.runs() {
					m.down(true)
				}
			}
			return nil
		},
	}
}

// value: an event that ends a Mac's engine process: its successor adopts
// what still runs.
func restarts(ev world.Event, mac string) bool {
	switch ev.Name {
	case "reboot", "power-cut", "engine-crash", "unlock":
		return len(ev.Args) > 0 && ev.Args[0] == mac
	}
	return false
}

// invariant: once the command is done, every engine of the world takes its
// turn on it, as the real ones run beside every command, then the world is
// written back.
func (r *worldRun) close(ctx context.Context) error {
	for _, name := range r.names() {
		m := r.mac(name)
		if _, ok := m.up(ctx); ok {
			m.settle(ctx)
			m.down(false)
		}
	}
	r.w.Lock()
	data, err := r.w.Encode()
	r.w.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o600)
}

type worldEnv struct {
	system.Env
	here, user string
}

func (e worldEnv) Hostname() string { return e.here }
func (e worldEnv) User() string     { return e.user }

// invariant: the world's clock moves only when the story moves it: a
// command that waits, a stepped engine's tick, an advance. What an engine
// runs in its own lifetime (a guest session's backoff) waits for the clock
// instead of moving it, so no loop of its own runs the clock ahead.
type worldClock struct {
	w     *world.World
	moved *moves
}

type waits struct{}

type moves struct {
	mu sync.Mutex
	ch chan struct{}
}

func (r *worldRun) clock() worldClock {
	if r.moved == nil {
		r.moved = &moves{ch: make(chan struct{})}
	}
	return worldClock{w: r.w, moved: r.moved}
}

func (m *moves) next() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ch
}

func (m *moves) tell() {
	m.mu.Lock()
	close(m.ch)
	m.ch = make(chan struct{})
	m.mu.Unlock()
}

func (c worldClock) Now() time.Time {
	c.w.Lock()
	defer c.w.Unlock()
	return c.w.Now
}

func (c worldClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Value(waits{}) == nil {
		// invariant: what already runs takes its turn before the clock
		// leaps, and a moment of real time with it (a fiftieth of the leap,
		// at most 5 ms), so a poll counts the work it waits on, a guest's
		// pipe under a loaded gate included, not the scheduler.
		runtime.Gosched()
		time.Sleep(min(d/50, 5*time.Millisecond))
	}
	c.w.Lock()
	until := c.w.Now.Add(d)
	if ctx.Value(waits{}) == nil {
		c.w.Now = until
		c.w.Unlock()
		c.moved.tell()
		return nil
	}
	c.w.Unlock()
	for {
		moved := c.moved.next()
		if !c.Now().Before(until) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-moved:
		}
	}
}

// invariant: a Mac's engine socket is its engine in this process, reached
// through a pipe; every other socket is the real one's.
type worldSockets struct {
	system.Sockets
	m *worldMac
}

var errEngineDown = errors.New("this Mac's engine does not run in the world")

// invariant: ssh's bytes reach the VM of this Mac that runs at ip and whose
// guest answers: its sshd greets with its name; any other address is no
// sshd. A world never dials the real network.
func (s worldSockets) Pipe(_ context.Context, ip string, port int, _ io.Reader, out io.Writer) error {
	w := s.m.run.w
	w.Lock()
	defer w.Unlock()
	for _, name := range slices.Sorted(maps.Keys(w.Mac(s.m.name).VMs)) {
		v := w.Mac(s.m.name).VMs[name]
		at := cmp.Or(v.Address, "192.168.64.5")
		if v.Running() && v.Guest != nil && at == ip {
			_, err := fmt.Fprintf(out, "SSH-2.0-world %s %s:%d\r\n", name, ip, port)
			return err
		}
	}
	return errNoSSHD
}

var errNoSSHD = errors.New("no sshd answers at this address in the world")

func (s worldSockets) DialUnix(ctx context.Context, _ string, _ time.Duration) (system.Conn, error) {
	d, ok := s.m.up(context.WithoutCancel(ctx))
	if !ok {
		return nil, errEngineDown
	}
	near, far := net.Pipe()
	go d.Serve(context.WithoutCancel(ctx), far)
	return near, nil
}

func (worldSockets) ListenUnix(string) (system.Listener, error) {
	return nil, errors.New("a world's engine is stepped, never listening")
}

type lockedNotifier struct {
	w *world.World
	n engine.Notifier
}

func (l lockedNotifier) Notify(ctx context.Context, n engine.Notification) {
	l.w.Lock()
	defer l.w.Unlock()
	l.n.Notify(ctx, n)
}

// invariant: a Mac of the world runs nothing as root: a command that asks
// sudo, even inside a shell line, is refused and kept in the world.
type rootless struct {
	system.Runner
	w *world.World
}

var errRoot = errors.New("this world runs nothing as root")

func (r rootless) refuses(c system.Cmd) bool {
	line := strings.Join(append([]string{c.Name}, c.Args...), " ")
	if !slices.Contains(strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == ';' || r == '&' || r == '|' || r == '(' || r == '\'' || r == '"'
	}), "sudo") {
		return false
	}
	r.w.Lock()
	r.w.AsRoot = append(r.w.AsRoot, line)
	r.w.Unlock()
	return true
}

func (r rootless) Output(ctx context.Context, c system.Cmd) (system.Output, error) {
	if r.refuses(c) {
		return system.Output{ExitCode: 1, Stderr: errRoot.Error()}, errRoot
	}
	return r.Runner.Output(ctx, c)
}

func (r rootless) Attach(ctx context.Context, c system.Cmd, out system.Stream) (system.Attached, error) {
	if r.refuses(c) {
		return system.Attached{}, errRoot
	}
	return r.Runner.Attach(ctx, c, out)
}

func (r rootless) Start(c system.Cmd, logPath string) (int, error) {
	if r.refuses(c) {
		return 0, errRoot
	}
	return r.Runner.Start(c, logPath)
}
