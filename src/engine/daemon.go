package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/services"
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
	// value: the station binary ping names, the one a client runs actions with.
	Executable string
	// value: how long a write to a subscriber may block before it is dropped.
	Stall time.Duration
	// value: the lend records and the recall a lend past its until goes through.
	Lends Lends
	// value: the Notifier the engine tells, built over the engine's own log.
	Notifier func(log func(subject, text string)) Notifier
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

func (d *Daemon) Check() error {
	if d.o.Here != catalog.Host {
		return &HostOnlyError{Here: d.o.Here}
	}
	return nil
}

func (d *Daemon) policy() services.Policy {
	g, err := d.st.Services.Get()
	if err != nil {
		return services.PolicyOf(nil)
	}
	return services.PolicyOf(g)
}

func (d *Daemon) saveLeases() {
	_ = services.SaveLeases(context.Background(), d.st.Reservations, append([]services.Lease(nil), d.leases...))
}

func (d *Daemon) Run(ctx context.Context, note func(line string)) error {
	d.note = note
	// invariant: the socket's liveness is the lock: a daemon that answers ping makes
	// a second one exit; a socket file nobody answers is removed and made again.
	// The socket is made only once adoption and serve reconciliation are done,
	// so its file means the daemon is ready.
	if err := d.Check(); err != nil {
		return err
	}
	sock := SocketPath(d.o.State)
	if p, err := DialPath(d.h, sock).Ping(ctx); err == nil {
		return &RunningError{PID: p.PID}
	}
	// invariant: the clamshell is set back to normal before the socket exists,
	// so a killed daemon's mask is cleared by its successor (launchd restarts it).
	d.power.Reset()
	_ = d.h.FS.Remove(sock)
	if n := d.h.Run.Reap(ChildrenDir(d.o.State)); n > 0 {
		d.logf("children", "killed "+strconv.Itoa(n)+" process(es) a killed daemon left running")
	}
	if err := d.h.FS.MkdirAll(d.o.State, 0o700); err != nil {
		return err
	}
	d.started = d.h.Clock.Now().UTC().Truncate(time.Second)
	d.leases, _ = d.st.Reservations.List(ctx)
	d.refreshSelf(ctx)
	d.adopt(ctx)
	d.sweepLends(ctx)
	d.reconcileServe(ctx)
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
		go d.serveConn(ctx, conn)
	}
}

func (d *Daemon) serveConn(ctx context.Context, conn system.Conn) {
	defer func() { _ = conn.Close() }()
	write := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = conn.Write(append(data, '\n'))
		return err
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var r Request
		if json.Unmarshal(scanner.Bytes(), &r) == nil && r.V == 1 && r.Op == "subscribe" {
			if err := d.checkPages(r.Pages); err != nil {
				var op *OpError
				errors.As(err, &op)
				if write(Response{V: 1, ID: r.ID, Error: op}) != nil {
					return
				}
				continue
			}
			d.stream(ctx, conn, r, write)
			return
		}
		if write(d.answer(ctx, scanner.Bytes())) != nil {
			return
		}
	}
}

func fault(code, message, hint string) *OpError {
	return &OpError{Code: code, Message: message, Hint: hint}
}

func (d *Daemon) answer(ctx context.Context, line []byte) Response {
	var r Request
	if err := json.Unmarshal(line, &r); err != nil {
		return Response{V: 1, Error: fault("unknown_op", "the request is not JSON", "speak "+Schema)}
	}
	resp := Response{V: 1, ID: r.ID}
	if r.V != 1 {
		resp.Error = fault("unknown_op", "unknown protocol version "+strconv.Itoa(r.V), "speak "+Schema)
		return resp
	}
	result, err := d.do(ctx, r)
	if err != nil {
		var op *OpError
		if !errors.As(err, &op) {
			op = fault("internal", err.Error(), "station machines")
		}
		resp.Error = op
		return resp
	}
	resp.OK = true
	if result != nil {
		resp.Result, _ = json.Marshal(result)
	}
	return resp
}

func (d *Daemon) do(ctx context.Context, r Request) (any, error) {
	switch r.Op {
	case "ping":
		return Ping{Version: d.o.Version, PID: d.o.PID, StartedAt: d.started, Executable: d.o.Executable}, nil
	case "vms":
		return d.vmViews(), nil
	case "services":
		return d.hostView(""), nil
	case "reserve":
		return d.reserve(ctx, r.VM, r.GuestPort, r.Pinned, r.Profile)
	case "release":
		return d.release(ctx, r.VM, r.GuestPort)
	case "run":
		if err := d.opRun(ctx, r.VM); err != nil {
			return nil, err
		}
		return nil, d.awaitGuest(ctx, r.VM)
	case "start":
		if err := d.opStart(ctx, r.VM); err != nil {
			return nil, err
		}
		return nil, d.awaitGuest(ctx, r.VM)
	case "stop":
		return nil, d.opStop(ctx, r.VM, true)
	case "restart":
		if err := d.opStop(ctx, r.VM, false); err != nil {
			return nil, err
		}
		if err := d.opRun(ctx, r.VM); err != nil {
			return nil, err
		}
		return nil, d.awaitGuest(ctx, r.VM)
	case "delete":
		return nil, d.opDelete(ctx, r.VM)
	case "forget":
		d.forget(r.VM)
		return nil, nil
	case "accounts":
		return d.accounts(ctx), nil
	case "guest_exec":
		return d.opGuestExec(ctx, r)
	case "stop_all":
		vms := d.stopAll(ctx)
		return StopAll{VMs: vms, PowerEnded: d.power.Stop()}, nil
	case "power":
		return d.power.View(), nil
	case "power_awake":
		return d.powerAwake(r)
	case "power_sleep":
		return d.powerSleep(), nil
	case "refresh":
		if err := d.checkPages(r.Pages); err != nil {
			return nil, err
		}
		d.hub.refreshed()
		return struct{}{}, nil
	}
	return nil, fault("unknown_op", "unknown op \""+r.Op+"\"", "speak "+Schema+": ping vms services reserve release run start stop restart delete accounts guest_exec stop_all power power_awake power_sleep subscribe refresh")
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

// invariant: stop_all ends every VM of this host at once: a temporary one is
// deleted, a running lasting one stopped with the wish stopped (its disk
// kept); each answers its own result, so a VM that would not stop keeps the
// daemon that supervises it.
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
		if r.Lifetime != machines.Temporary && !running {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := Stopped{Name: r.Name, Lifetime: string(r.Lifetime), Result: ResultStopped}
			var err error
			if r.Lifetime == machines.Temporary {
				s.Result = ResultDeleted
				err = d.opDelete(ctx, r.Name)
			} else {
				err = d.opStop(ctx, r.Name, true)
				if err == nil && d.m.EngineRunning(ctx, r.Name) {
					err = errors.New("the engine still runs it")
				}
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
