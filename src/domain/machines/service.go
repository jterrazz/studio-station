package machines

import (
	"context"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"path"
	"strings"
	"time"

	background "github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

type Ports struct{ OpenClaw, Hermes int }

type Options struct {
	Ports     Ports
	Here      catalog.MachineKind
	Paths     settings.Paths
	User      string
	Version   string
	Remote    func(target string, words []string) []string
	GuestPort int
	Jobs      background.Jobs
	Unlocker  contract.Unlocker
	SSHConfig contract.SSHConfig
}

type Node struct {
	Online bool
	IP     string
}

type Nodes interface {
	Peers(ctx context.Context) (map[string]Node, error)
}

type DaemonVM struct{ State, Guest, IP, Detail string }

type Reserved struct {
	GuestPort, HostPort int
	Profile, URL        string
	PublicHost          string
	ServeOff            bool
}

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
	store  settings.Store
	nodes  Nodes
	img    Images
	daemon Daemon
	rel    Releaser
	o      Options
}

func New(h system.Host, e contract.Hypervisor, store settings.Store, nodes Nodes, img Images, d Daemon, rel Releaser, o Options) *Service {
	return &Service{h: h, engine: e, store: store, nodes: nodes, img: img, daemon: d, rel: rel, o: o}
}

type Lifetime = model.Lifetime

const (
	Temporary = model.Temporary
	Lasting   = model.Lasting
)

type Resources struct {
	CPUs        int
	MemoryBytes int64
	DiskBytes   int64
}

func (r Resources) size() model.Size {
	return model.Size{CPUs: r.CPUs, Memory: r.MemoryBytes, Disk: r.DiskBytes}
}

type VMState string

const (
	Running   VMState = "running"
	Stopped   VMState = "stopped"
	Suspended VMState = "suspended"
	Stale     VMState = "stale"
	Broken    VMState = "broken"
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

type VM struct {
	Name, Image, Version, Host string
	Kind                       catalog.MachineKind
	Lifetime                   Lifetime
	State                      VMState
	Resources                  Resources
	SizeGB                     int
	IP, Network, Guest, Detail string
	Lent                       []string
	CreatedAt                  time.Time
}

type DaemonInfo struct {
	Known, Running bool
	// value: the agent's plist is on this host: a daemon not running is
	// stopped, not missing (R9.D3); unknown for another host.
	Installed bool
	Version   string
}

type HostView struct {
	Name, User, StationVersion string
	Here, Reachable            bool
	Node                       Node
	Daemon                     DaemonInfo
	VMs                        []VM
}

type Fleet struct{ Hosts []HostView }

type Check struct {
	Name          string
	State         catalog.State
	Value, Detail string
}

type ProbeReport struct {
	Name, User, IP string
	Checks         []Check
	Console        string
}

func (s *Service) vmUser(cfg settings.Settings) string {
	if cfg.VMUser != "" {
		return cfg.VMUser
	}
	return s.o.User
}

func (s *Service) recordsDir() string { return path.Join(s.o.Paths.State, "vms") }

func KnownHostsDir(p settings.Paths) string { return path.Join(p.State, "known_hosts") }

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
