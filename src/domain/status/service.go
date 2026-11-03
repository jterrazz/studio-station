package status

import (
	"context"
	"path"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	schedule "github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/system"
)

type Options struct {
	Pool                     *schedule.Pool
	Root                     string
	Here                     catalog.MachineKind
	OpenClawPort, HermesPort int
	Version, Commit          string
}

type Sources struct {
	Tools   ToolsReader
	Config  ConfigLister
	Tailnet TailnetReader
	Setup   SetupChecker
	Store   settings.Store
	Storage StorageReader
	Daemon  DaemonReader
	Power   PowerReader
}

type PowerReader interface {
	// value: the daemon's Power session on this host; ok false where Power
	// cannot be read (a VM, no power management, the daemon down).
	Session(ctx context.Context) (s PowerState, ok bool)
}

type PowerState struct {
	Held, Lid bool
	Until     time.Time
}

type DaemonReader interface {
	Ping(ctx context.Context) (version string, err error)
}

type StorageReader interface {
	Watch(ctx context.Context) <-chan storage.Report
}

type ToolsReader interface {
	Inventory(ctx context.Context) ([]tools.Report, error)
}

type ConfigLister interface {
	List(ctx context.Context) ([]config.Report, error)
}

type TailnetReader interface {
	Status(ctx context.Context) (tailnet.Status, error)
}

type SetupChecker interface {
	Check(ctx context.Context) (setup.Report, error)
}

type Service struct {
	h    system.Host
	deps Sources
	o    Options
}

func New(h system.Host, deps Sources, o Options) *Service {
	if o.Pool == nil {
		o.Pool = schedule.NewPool(context.Background(), h.Clock, commandLane, 1)
	}
	return &Service{h: h, deps: deps, o: o}
}

type Snapshot struct {
	GeneratedAt time.Time
	Station     Station
	Machine     Machine
	System      System
	Environment Environment
	Workspace   Workspace
	Tools       []tools.Report
	Config      []config.Report
	Setup       Setup
	Identity    []Check
	Background  []Job
	Gateways    []Check
	Tailnet     tailnet.Status
	Power       PowerState
	// value: Power was read (a host that has it, its daemon answering).
	PowerKnown bool
	// value: a section whose reading has not answered; never past Snapshot.
	ToolsPending, ConfigPending bool
	found                       []Check
	err                         error
}

type Station struct {
	Version, Commit string
	Daemon          Check
}

type Machine struct {
	Name       string
	Registered bool
	Kind       catalog.MachineKind
	Hostname   string
	OS         string
	Arch       string
	User       string
	Shell      string
}

type Check struct {
	Name   string
	State  catalog.State
	Value  string
	Detail string
}

type Setup struct {
	State catalog.State
	Steps []setup.Step
}

type Process struct {
	PID   int
	Name  string
	Value float64
}

type Sample struct {
	CPU, Memory  []Process
	NetRx, NetTx int64
	GPU          float64
	At           time.Time
}

type System struct {
	Uptime      string
	Battery     string
	MemoryTotal int64
	Sample      Sample
	Pending     bool
}

type Port struct {
	PID  int
	Name string
	Port string
}

type Environment struct {
	Network  []Check
	Ports    []Port
	Security []Check
	Updates  []Check
}

type Repository struct {
	Path   string
	Branch string
	Dirty  int
}

type Dependency struct {
	Path     string
	Manager  string
	Outdated int
}

type Docker struct {
	State                       catalog.State
	Containers, Images, Volumes int
	DanglingBytes, TotalBytes   int64
}

type Disk struct {
	Name  string
	Level catalog.Level
	Bytes int64
}

type Workspace struct {
	Repositories []Repository
	Dependencies []Dependency
	Docker       Docker
	Disk         []Disk
	Strays       []storage.StrayReport
	// value: a row whose reading has not answered; never past Snapshot.
	ReposPending, DepsPending, DiskPending bool
}

func (s *Service) mac() bool { return s.o.Here != catalog.Linux }

func (s Snapshot) Err() error { return s.err }

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	snap := schedule.Last(s.Watch(ctx))
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	return snap, snap.err
}

// invariant: a protection a config item installs is read from that item's own state,
// so status and `station config` can never disagree about it.
func fromConfig(reports []config.Report, item, name string) (Check, bool) {
	for _, r := range reports {
		if r.Item.Name != item {
			continue
		}
		switch r.State {
		case catalog.StateOK:
			return Check{Name: name, State: catalog.StateOK, Value: "on"}, true
		case catalog.StateAbsent:
			return Check{Name: name, State: catalog.StateAbsent, Value: "off"}, true
		case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown, catalog.StatePending:
		}
		return Check{Name: name, State: catalog.StateUnknown}, true
	}
	return Check{}, false
}

func (s *Service) setup(ctx context.Context) Setup {
	report, err := s.deps.Setup.Check(ctx)
	if err != nil {
		return Setup{State: catalog.StateUnknown, Steps: []setup.Step{}}
	}
	state := catalog.StateOK
	for _, step := range report.Steps {
		if worse(step.State, state) {
			state = step.State
		}
	}
	steps := report.Steps
	if steps == nil {
		steps = []setup.Step{}
	}
	return Setup{State: state, Steps: steps}
}

func rank(s catalog.State) int {
	switch s {
	case catalog.StateOK:
		return 0
	case catalog.StateUnknown:
		return 1
	case catalog.StateDegraded:
		return 2
	case catalog.StateAbsent:
		return 3
	case catalog.StateBroken:
		return 4
	case catalog.StatePending:
	}
	return 1
}

func worse(a, than catalog.State) bool { return rank(a) > rank(than) }

func shellName(shell string) string {
	if shell == "" {
		return ""
	}
	return path.Base(shell)
}

func (s *Service) agentInstalled() bool {
	_, err := s.h.FS.Stat(catalog.DaemonPlist(s.h.Env.Home()))
	return err == nil
}

// invariant: the daemon matters where VMs can exist: a host with tart; one that
// does not answer, or answers another version, is degraded, never an error.
func (s *Service) daemon(ctx context.Context) Check {
	c := Check{Name: "daemon", State: catalog.StateAbsent}
	if s.o.Here != catalog.Host || s.deps.Daemon == nil {
		return c
	}
	if _, ok := s.h.Run.LookPath("tart"); !ok {
		c.Value = "not needed until tart is installed"
		return c
	}
	version, err := s.deps.Daemon.Ping(ctx)
	switch {
	case err != nil && s.agentInstalled():
		c.State, c.Value, c.Detail = catalog.StateDegraded, "stopped", "station start"
	case err != nil:
		c.State, c.Value, c.Detail = catalog.StateDegraded, "not installed", "station setup"
	case version != s.o.Version:
		c.State, c.Value, c.Detail = catalog.StateDegraded, "runs "+version+", station is "+s.o.Version, "station setup"
	default:
		c.State, c.Value = catalog.StateOK, "running "+version
	}
	return c
}
