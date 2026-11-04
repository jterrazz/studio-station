package machines

import (
	"context"
	"strings"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	background "github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type Ports struct{ OpenClaw, Hermes int }

type Options struct {
	Ports Ports
	Here  catalog.MachineKind
	// value: the folders of the VMs' known hosts and of their logs.
	KnownHosts, Logs string
	User             string
	Version          string
	// value: asks each other Mac for its own view of itself and hands on
	// each answer as it comes, one at a time; a silent Mac is never handed on.
	Ask       func(ctx context.Context, macs []string, answered func(mac string, view HostView))
	GuestPort int
	Jobs      background.Jobs
	Unlocker  contract.Unlocker
	SSHConfig contract.SSHConfig
}

type Nodes interface {
	Peers(ctx context.Context) (map[string]Node, error)
}

type DaemonVM struct{ State, Guest, IP, Detail string }

type Daemon interface {
	Ping(ctx context.Context) (version string, err error)
	VMs(ctx context.Context) (map[string]DaemonVM, error)
	Do(ctx context.Context, op, vm string) error
	Reserve(ctx context.Context, vm string, port int, profile string) (Reserved, error)
	// invariant: a command inside a VM of this Mac rides the engine's one
	// connection to it; station reaches a VM no other way.
	GuestExec(ctx context.Context, vm, line, stdin string) (system.Output, error)
}

type Releaser interface {
	ReleaseVM(ctx context.Context, vm string) error
}

type Images interface {
	Latest(ctx context.Context, image string) (disk, version string, err error)
	KeyPath() string
}

type Service struct {
	h      system.Host
	engine contract.Hypervisor
	p      Prefs
	recs   contract.MachineStore
	nodes  Nodes
	img    Images
	daemon Daemon
	rel    Releaser
	o      Options
}

type Prefs struct {
	Name   setupcontract.NamePrefs
	Hosts  contract.MacsPrefs
	VMUser contract.VMUserPrefs
}

type prefsView struct {
	Here   string
	Hosts  map[string]model.Host
	VMUser string
}

func (s *Service) prefs() (prefsView, error) {
	var v prefsView
	var err error
	if v.Here, err = s.p.Name.Get(); err != nil {
		return prefsView{}, err
	}
	if v.Hosts, err = s.p.Hosts.Get(); err != nil {
		return prefsView{}, err
	}
	if v.VMUser, err = s.p.VMUser.Get(); err != nil {
		return prefsView{}, err
	}
	return v, nil
}

func New(h system.Host, e contract.Hypervisor, p Prefs, vms contract.MachineStore, nodes Nodes, img Images, d Daemon, rel Releaser, o Options) *Service {
	return &Service{h: h, engine: e, p: p, recs: vms, nodes: nodes, img: img, daemon: d, rel: rel, o: o}
}

type Lifetime = model.Lifetime

const (
	Throwaway = model.Throwaway
	Permanent = model.Permanent
)

const (
	WantRunning = "running"
	WantStopped = "stopped"
	NetworkNAT  = "nat"
	Bridged     = "bridged"
)

type VMRecord struct {
	Name, Image, Version string
	Lifetime             Lifetime
	Want, Network        string
	Env                  map[string]string
	Resources            Resources
	Lent                 []string
	CreatedAt            time.Time
}

type Machine struct {
	Name      string
	Kind      catalog.MachineKind
	Here      bool
	Host      string
	User, Key string
	Lifetime  Lifetime
}

type (
	Node        = model.Node
	Reserved    = model.Reserved
	Resources   = model.Resources
	VMState     = model.VMState
	VM          = model.VM
	DaemonInfo  = model.EngineView
	HostView    = model.HostView
	Fleet       = model.Fleet
	Check       = model.Check
	ProbeReport = model.ProbeReport
	SSHTarget   = model.SSHTarget
)

const (
	Running   = model.Running
	Stopped   = model.Stopped
	Suspended = model.Suspended
	Stale     = model.Stale
	Broken    = model.Broken
)

func (s *Service) vmUser(cfg prefsView) string {
	if cfg.VMUser != "" {
		return cfg.VMUser
	}
	return s.o.User
}

func (s *Service) CheckHost() error {
	if s.o.Here != catalog.Host {
		return &OnlyOnHostError{Here: s.o.Here}
	}
	return nil
}

var nameChars = func(r rune) bool { return r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') }

func validName(name string) bool {
	return len(name) >= 1 && len(name) <= 40 && strings.IndexFunc(name, func(r rune) bool { return !nameChars(r) }) < 0
}
