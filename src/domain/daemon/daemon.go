package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	stopWait   = 60 * time.Second
	hardWait   = 10 * time.Second
	pollEvery  = 200 * time.Millisecond
	selfEvery  = 30
	seenEvery  = time.Minute
	guestReady = 60 * time.Second
)

type Machines interface {
	Records() ([]machines.VMRecord, error)
	Save(machines.VMRecord) error
	Remove(ctx context.Context, name string) (machines.DeleteReport, error)
	PowerOff(ctx context.Context, name string) error
	EngineRunning(ctx context.Context, name string) bool
	DeclaredPoweroff(console string) bool
	KeyPath() string
	VMUser() string
	HereName() string
	ConsolePath(name string) string
	LogPath(name string) string
	KnownHostsPath(name string) string
}

type Tailnet interface {
	Self(ctx context.Context) (tailnet.Self, error)
	Serve(ctx context.Context, args ...string) error
	ServeStatus(ctx context.Context) (tailnet.ServeConfig, error)
}

type Options struct {
	Paths     settings.Paths
	Here      catalog.MachineKind
	Version   string
	PID       int
	Tick      time.Duration
	GuestPort int
	Log       io.Writer
}

type Daemon struct {
	h      system.Host
	engine system.Engine
	ssh    system.SSH
	m      Machines
	tn     Tailnet
	store  settings.Store
	o      Options

	mu      sync.Mutex
	vms     map[string]*vmState
	svcs    map[string]*svc
	leases  []services.Lease
	self    tailnet.Self
	started time.Time
}

func New(h system.Host, e system.Engine, ssh system.SSH, m Machines, tn Tailnet, store settings.Store, o Options) *Daemon {
	if o.Tick == 0 {
		o.Tick = time.Second
	}
	if o.GuestPort == 0 {
		o.GuestPort = 22
	}
	if o.Log == nil {
		o.Log = io.Discard
	}
	return &Daemon{h: h, engine: e, ssh: ssh, m: m, tn: tn, store: store, o: o, vms: map[string]*vmState{}, svcs: map[string]*svc{}}
}

func (d *Daemon) Check() error {
	if d.o.Here != catalog.Host {
		return &HostOnlyError{}
	}
	return nil
}

func (d *Daemon) policy() services.Policy {
	cfg, err := d.store.Load()
	if err != nil {
		return services.PolicyOf(nil)
	}
	return services.PolicyOf(cfg.Services)
}

func (d *Daemon) leaseStore() services.Store { return services.NewStore(d.h.FS, d.o.Paths.State) }

func (d *Daemon) saveLeases() { _ = d.leaseStore().Save(append([]services.Lease(nil), d.leases...)) }

func (d *Daemon) Run(ctx context.Context) error {
	// invariant: the socket's liveness is the lock: a daemon that answers ping makes
	// a second one exit; a socket file nobody answers is removed and made again.
	// The socket is made only once adoption and serve reconciliation are done,
	// so its file means the daemon is ready.
	if err := d.Check(); err != nil {
		return err
	}
	sock := SocketPath(d.o.Paths)
	if p, err := DialPath(d.h, sock).Ping(ctx); err == nil {
		return &RunningError{PID: p.PID}
	}
	_ = d.h.FS.Remove(sock)
	if err := d.h.FS.MkdirAll(d.o.Paths.State, 0o700); err != nil {
		return err
	}
	d.started = d.h.Clock.Now().UTC().Truncate(time.Second)
	d.leases, _ = d.leaseStore().Load()
	d.refreshSelf(ctx)
	d.adopt(ctx)
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
	for tick := 1; ; tick++ {
		if err := d.h.Clock.Sleep(ctx, d.o.Tick); err != nil {
			d.closeAll()
			return nil
		}
		if tick%selfEvery == 0 {
			d.refreshSelf(ctx)
		}
		d.tick(ctx)
	}
}

func (d *Daemon) logf(subject, message string) {
	_, _ = io.WriteString(d.o.Log, d.h.Clock.Now().UTC().Format(time.RFC3339)+" "+subject+": "+message+"\n")
}

func (d *Daemon) refreshSelf(ctx context.Context) {
	self, err := d.tn.Self(ctx)
	if err != nil {
		self = tailnet.Self{}
	}
	d.mu.Lock()
	d.self = self
	d.mu.Unlock()
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
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		resp := d.answer(ctx, scanner.Bytes())
		data, err := json.Marshal(resp)
		if err != nil {
			return
		}
		if _, err := conn.Write(append(data, '\n')); err != nil {
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
		return Ping{Version: d.o.Version, PID: d.o.PID, StartedAt: d.started}, nil
	case "vms":
		return d.vmViews(), nil
	case "services":
		return d.hostView(""), nil
	case "reserve":
		return d.reserve(ctx, r.VM, r.GuestPort, r.Pinned, r.Profile)
	case "release":
		return d.release(ctx, r.VM, r.GuestPort)
	case "run":
		return nil, d.opRun(ctx, r.VM)
	case "start":
		return nil, d.opStart(ctx, r.VM)
	case "stop":
		return nil, d.opStop(ctx, r.VM, true)
	case "restart":
		if err := d.opStop(ctx, r.VM, false); err != nil {
			return nil, err
		}
		return nil, d.opRun(ctx, r.VM)
	case "delete":
		return nil, d.opDelete(ctx, r.VM)
	case "forget":
		d.forget(r.VM)
		return nil, nil
	}
	return nil, fault("unknown_op", "unknown op \""+r.Op+"\"", "speak "+Schema+": ping vms services reserve release run start stop restart delete")
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
