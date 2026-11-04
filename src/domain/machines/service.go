package machines

import (
	"context"
	"path"
	"strings"
	"time"

	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

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
	Hop       func(host string, words []string) []string
	GuestPort int
	Jobs      background.Jobs
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
	Hosts  contract.HostsPrefs
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

func (s *Service) vmUser(cfg prefsView) string {
	if cfg.VMUser != "" {
		return cfg.VMUser
	}
	return s.o.User
}

func (s *Service) knownHostsDir() string { return path.Join(s.o.Paths.State, "known_hosts") }

func (s *Service) sshConfig() string { return path.Join(s.h.Env.Home(), ".ssh", "config") }

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
