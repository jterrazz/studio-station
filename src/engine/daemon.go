package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	stopWait   = 60 * time.Second
	hardWait   = 10 * time.Second
	pollEvery  = 200 * time.Millisecond
	serveEvery = 30
	seenEvery  = time.Minute
	guestReady = 60 * time.Second
	certBudget = 120 * time.Second
)

type Machines interface {
	Records() ([]machines.VMRecord, error)
	Save(machines.VMRecord) error
	Remove(ctx context.Context, name string) (machines.DeleteReport, error)
	PowerOff(ctx context.Context, name string) error
	EngineRunning(ctx context.Context, name string) bool
	HereName() string
}

type Tailnet interface {
	Ready(ctx context.Context) (ports.SharingState, error)
	Share(ctx context.Context, s ports.Share) (string, error)
	Unshare(ctx context.Context, s ports.Share) error
	Shared(ctx context.Context) ([]ports.Share, error)
	Warm(ctx context.Context) error
}

type Options struct {
	// value: keeps whether the owner stopped sharing a VM's port, so the
	// next listen there is not shared until a share; nil keeps nothing.
	Remember func(vm string, port int, stopped bool) error
	// value: the engine's own state folder: its socket and its children.
	State     string
	Here      catalog.MachineKind
	Version   string
	PID       int
	Tick      time.Duration
	GuestPort int
	// value: this Mac's power, whose sessions the daemon holds; nil is none.
	Power power.Port
	// value: the power page a subscription streams, built by the caller's pages.
	PowerPage func(power.View) any
	// value: the other pages a subscription streams (the menu bar's), by id.
	Feeds map[string]Feed
	// value: the station binary hello names, the one a terminal opens.
	Executable string
	// value: this engine as a client sees it: a second engine asks it first.
	Peer Peer
	// value: the lend records and the recall a lend past its until goes through.
	Lends Lends
	// value: the Notifier the engine tells, built over the engine's own log.
	Notifier func(log func(subject, text string)) Notifier
	// value: what runs a server's health check from this Mac; nil runs none.
	Checks HealthProbe
	// value: how the parts of what the engine tells are joined, as the
	// presentation joins them.
	Join func(parts ...string) string
	// value: when the engine process started, for a stepped engine that
	// stands for one which outlived its command; zero is the Begin.
	StartedAt time.Time
}

type Daemon struct {
	h      system.Host
	hv     Hypervisor
	guests Guests
	m      Machines
	tn     Tailnet
	st     Stores
	o      Options

	mu      sync.Mutex
	vms     map[string]*vmState
	svcs    map[string]*svc
	leases  []services.Lease
	self    ports.SharingState
	read    bool
	issuing bool
	started time.Time
	note    func(line string)
	power   *power.Machine
	hub     hub

	notifier Notifier
	serve    func(ctx context.Context, conn io.ReadWriteCloser)
	memory   Memory
	// value: the engine's own lifetime, from Begin: the guest sessions it
	// holds live in it, never in the request that started a VM.
	life context.Context

	lendMu       sync.Mutex
	lendsAt      time.Time
	recallFailed map[string]time.Time
}

type Stores struct {
	Services     Policy
	Reservations Reservations
}

type Policy interface {
	Get() (*ports.PortsGroup, error)
}

type Reservations interface {
	// value: the reserved ports, kept by their store; the engine is their one
	// writer while it runs.
	Get(ctx context.Context, vm string, guestPort int) (ports.Reservation, error)
	Put(ctx context.Context, r ports.Reservation) error
	List(ctx context.Context) ([]ports.Reservation, error)
	Delete(ctx context.Context, vm string, guestPort int) error
}

func New(h system.Host, hv Hypervisor, guests Guests, m Machines, tn Tailnet, st Stores, o Options) *Daemon {
	if o.Tick == 0 {
		o.Tick = time.Second
	}
	if o.GuestPort == 0 {
		o.GuestPort = 22
	}
	d := &Daemon{h: h, hv: hv, guests: guests, m: m, tn: tn, st: st, o: o, vms: map[string]*vmState{}, svcs: map[string]*svc{}, recallFailed: map[string]time.Time{}}
	d.power = d.newPower()
	if o.Notifier != nil {
		d.notifier = o.Notifier(d.logf)
	}
	return d
}

func (d *Daemon) Host(serve func(ctx context.Context, conn io.ReadWriteCloser)) { d.serve = serve }

func (d *Daemon) Check() error {
	// value: the server of station.api/v2, run on each connection of the
	// socket; it is the engine's, built over this daemon, so it is handed after New.
	if d.o.Here != catalog.Host {
		return &HostOnlyError{Here: d.o.Here}
	}
	return nil
}

func (d *Daemon) policy() ports.Policy {
	g, err := d.st.Services.Get()
	if err != nil {
		return ports.PolicyOf(nil)
	}
	return ports.PolicyOf(g)
}

func (d *Daemon) saveLeases() {
	_ = services.SaveLeases(context.Background(), d.st.Reservations, append([]services.Lease(nil), d.leases...))
}

func (d *Daemon) Run(ctx context.Context, note func(line string)) error {
	// invariant: the socket's liveness is the lock: a daemon that answers ping makes
	// a second one exit; a socket file nobody answers is removed and made again.
	// The socket is made only once adoption and serve reconciliation are done,
	// so its file means the daemon is ready.
	if err := d.Check(); err != nil {
		return err
	}
	sock := SocketPath(d.o.State)
	if d.o.Peer != nil {
		if p, err := d.o.Peer.Hello(ctx); err == nil {
			return &RunningError{PID: p.PID}
		}
	}
	if err := d.Begin(ctx, note); err != nil {
		return err
	}
	_ = d.h.FS.Remove(sock)
	l, err := d.h.Sockets.ListenUnix(sock)
	if err != nil {
		return err
	}
	defer func() {
		_ = l.Close()
		_ = d.h.FS.Remove(sock)
	}()
	go d.accept(ctx, l)
	go d.watchPower(ctx)
	for tick := 1; ; tick++ {
		if err := d.h.Clock.Sleep(ctx, d.o.Tick); err != nil {
			d.power.Quit()
			d.closeAll()
			return nil
		}
		d.step(ctx, tick)
	}
}

func (d *Daemon) Begin(ctx context.Context, note func(line string)) error {
	// invariant: Begin is the engine up to its socket: the clamshell set back
	// to normal (a killed daemon's mask is cleared by its successor), the
	// children a killed one left reaped, the runs adopted, the lends swept and
	// Serve reconciled. Run follows it with the socket and a tick a second; a
	// stepped engine (the spec world) follows it with Step, then Serve per
	// connection, and Stop.
	if err := d.Check(); err != nil {
		return err
	}
	d.note, d.life = note, ctx
	d.power.Reset()
	if n := d.h.Run.Reap(ChildrenDir(d.o.State)); n > 0 {
		d.logf("children", "killed "+strconv.Itoa(n)+" process(es) a killed engine left running")
	}
	if err := d.h.FS.MkdirAll(d.o.State, 0o700); err != nil {
		return err
	}
	d.started = d.h.Clock.Now().UTC().Truncate(time.Second)
	if !d.o.StartedAt.IsZero() {
		d.started = d.o.StartedAt.UTC().Truncate(time.Second)
	}
	d.leases, _ = d.st.Reservations.List(ctx)
	d.refreshSelf(ctx)
	d.adopt(ctx)
	d.sweepLends(ctx)
	d.reconcileServe(ctx)
	return nil
}

func (d *Daemon) Step(ctx context.Context, tick int) bool {
	// value: one tick of a stepped engine; it reports whether it is busy: a VM
	// changed, a run waits for its delay, or a health check runs.
	before := d.vmViews()
	d.step(ctx, tick)
	return !reflect.DeepEqual(before, d.vmViews()) || d.waiting()
}

func (d *Daemon) waiting() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, v := range d.vms {
		if !v.rerunAt.IsZero() || v.checking {
			return true
		}
	}
	return false
}

type Memory map[string]Remembered

type Remembered struct {
	// value: what an engine knows of a VM beyond its record and its run:
	// its state and why, and the runs it started again; a stepped engine
	// hands it from one command to the next, as a real one keeps it between
	// ticks.
	State   string      `json:"state"`
	Detail  string      `json:"detail,omitempty"`
	Reruns  []time.Time `json:"reruns,omitempty"`
	RerunAt time.Time   `json:"rerun_at,omitzero"`
	// value: a server's restarts since this engine started, when its run
	// began, and its health check as last run.
	Restarts model.Restarts    `json:"restarts,omitzero"`
	BootAt   time.Time         `json:"boot_at,omitzero"`
	Check    model.CheckStatus `json:"check,omitzero"`
}

func (d *Daemon) Memory() Memory {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := Memory{}
	for name, v := range d.vms {
		out[name] = Remembered{State: v.state, Detail: v.detail, Reruns: v.reruns, RerunAt: v.rerunAt, Restarts: v.restarts, BootAt: v.bootAt, Check: v.check}
	}
	return out
}

func (d *Daemon) Remember(m Memory) {
	// invariant: Remember comes before Begin: a VM it holds broken, or waiting
	// for its delay, is not run again by the adoption.
	d.memory = m
}

func (d *Daemon) Serve(ctx context.Context, conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	if d.serve != nil {
		d.serve(ctx, conn)
	}
}

func (d *Daemon) Stop() {
	d.power.Quit()
	d.closeAll()
}

func (d *Daemon) step(ctx context.Context, tick int) {
	if tick%serveEvery == 0 {
		d.refreshSelf(ctx)
	}
	d.tick(ctx)
	d.sweepLends(ctx)
	d.power.Tick()
	d.watchState()
}

func (d *Daemon) logf(subject, message string) {
	if d.note != nil {
		d.note(d.h.Clock.Now().UTC().Format(time.RFC3339) + " " + subject + ": " + message)
	}
}

// invariant: Serve's state is read at start and every serveEvery ticks: on,
// every published lease is served and the entries no lease justifies go; off,
// no serve runs and every row says so, logged once per change.
func (d *Daemon) refreshSelf(ctx context.Context) {
	self, err := d.tn.Ready(ctx)
	if err != nil {
		self = ports.SharingState{}
	}
	d.mu.Lock()
	was, first := d.self.On, !d.read
	d.self, d.read = self, true
	d.mu.Unlock()
	if !first && self.On == was {
		return
	}
	if self.On {
		d.warmCert(ctx, self.Host)
	}
	switch {
	case self.On && !first:
		d.logf("tailnet", "Serve is on: every lease is served on the tailnet")
		d.serveAll(ctx)
		d.reconcileServe(ctx)
	case !self.On:
		d.logf("tailnet", catalog.ServeOffDetail+": services are local only")
		d.unserveAll()
	}
}

// invariant: the tailnet issues the host's certificate at the first https
// request, 34.5 s measured, so the daemon asks for it as Serve turns on, in
// the background: never blocking a publish, one at a time, within its budget.
func (d *Daemon) warmCert(ctx context.Context, host string) {
	d.mu.Lock()
	if d.issuing || host == "" {
		d.mu.Unlock()
		return
	}
	d.issuing = true
	d.mu.Unlock()
	go func() {
		start := d.h.Clock.Now()
		wait, cancel := context.WithDeadline(ctx, start.Add(certBudget))
		err := d.tn.Warm(wait)
		cancel()
		d.mu.Lock()
		d.issuing = false
		d.mu.Unlock()
		switch {
		case ctx.Err() != nil:
		case err == nil:
			d.logf("tailnet", "certificate for "+host+" ready in "+d.h.Clock.Now().Sub(start).Round(time.Millisecond).String())
		case errors.Is(err, system.ErrTimeout) || errors.Is(err, context.DeadlineExceeded):
			d.logf("tailnet", "certificate for "+host+" not issued: no answer within "+certBudget.String())
		default:
			d.logf("tailnet", "certificate for "+host+" not issued: "+err.Error())
		}
	}()
}

func (d *Daemon) accept(ctx context.Context, l system.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go d.Serve(ctx, conn)
	}
}

type VMAccounts struct {
	Name     string          `json:"name"`
	Guest    string          `json:"guest"`
	Accounts json.RawMessage `json:"accounts,omitempty"`
}

type AccountsView struct {
	VMs []VMAccounts `json:"vms"`
}

const accountsBudget = 12 * time.Second

// invariant: each running VM is asked by its own guest, all at once, each
// within accountsBudget; a VM whose guest is not connected is "absent" with
// no rows. Only the guest's parsed rows travel: never a provider's output.
func (d *Daemon) accounts(ctx context.Context) AccountsView {
	d.mu.Lock()
	var running []*vmState
	for _, v := range d.vms {
		if v.state == StateRunning {
			running = append(running, v)
		}
	}
	d.mu.Unlock()
	out := make([]VMAccounts, len(running))
	var wg sync.WaitGroup
	for i, v := range running {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.mu.Lock()
			guest := v.guest
			d.mu.Unlock()
			out[i] = VMAccounts{Name: v.rec.Name, Guest: GuestAbsent}
			if guest != GuestConnected {
				return
			}
			rows, err := d.askGuest(ctx, v, "accounts", accountsBudget)
			if err != nil {
				d.logf(v.rec.Name+" accounts", err.Error())
				return
			}
			out[i] = VMAccounts{Name: v.rec.Name, Guest: GuestConnected, Accounts: rows}
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return AccountsView{VMs: out}
}

type Stopped struct {
	Name     string `json:"name"`
	Lifetime string `json:"lifetime"`
	Result   string `json:"result"`
	Detail   string `json:"detail,omitempty"`
}

const (
	ResultDeleted = "deleted"
	ResultStopped = "stopped"
	ResultFailed  = "failed"
)

// invariant: stop_all stops every VM of this host that runs, at once, with
// the wish stopped and its disk kept, whatever its lifetime: only a delete
// destroys. Each answers its own result, so a VM that would not stop keeps
// the engine that supervises it.
func (d *Daemon) stopAll(ctx context.Context) []Stopped {
	records, _ := d.m.Records()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var out []Stopped
	for _, r := range records {
		v, err := d.find(r.Name)
		if err != nil {
			continue
		}
		d.mu.Lock()
		running := v.state == StateRunning
		d.mu.Unlock()
		if !running {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := Stopped{Name: r.Name, Lifetime: string(r.Lifetime), Result: ResultStopped}
			err := d.halt(ctx, v, true)
			if err == nil && d.m.EngineRunning(ctx, r.Name) {
				err = errors.New("the engine still runs it")
			}
			if err != nil {
				s.Result, s.Detail = ResultFailed, err.Error()
				d.logf(r.Name, "stop_all: "+err.Error())
			}
			mu.Lock()
			out = append(out, s)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (d *Daemon) sharedVMs() map[string]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]bool{}
	for _, l := range d.leases {
		out[l.VM] = true
	}
	return out
}

func (d *Daemon) closeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.svcs {
		for _, l := range s.listeners {
			_ = l.Close()
		}
		s.listeners = nil
	}
	for _, v := range d.vms {
		if v.cancel != nil {
			v.cancel()
		}
	}
}

func (d *Daemon) hostKeys() []string {
	home := d.h.Env.Home()
	var keys []string
	if pub, err := d.h.FS.ReadFile(path.Join(home, ".ssh", "id_ed25519.pub")); err == nil {
		if line := strings.TrimSpace(string(pub)); line != "" {
			keys = append(keys, line)
		}
	}
	if auth, err := d.h.FS.ReadFile(path.Join(home, ".ssh", "authorized_keys")); err == nil {
		for _, line := range strings.Split(string(auth), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				keys = append(keys, line)
			}
		}
	}
	return keys
}
