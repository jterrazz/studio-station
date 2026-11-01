package status

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/system"
)

type Options struct {
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
}

type DaemonReader interface {
	Ping(ctx context.Context) (version string, err error)
}

type StorageReader interface {
	List(ctx context.Context) (storage.Report, error)
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
	Daemons     []Check
	Gateways    []Check
	Tailnet     tailnet.Status
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
}

func (s *Service) mac() bool { return s.o.Here != catalog.Linux }

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	// invariant: every part runs at once under its own budget, so a slow tool delays
	// only its own row.
	snap := Snapshot{
		GeneratedAt: s.h.Clock.Now().UTC(),
		Station:     Station{Version: s.o.Version, Commit: s.o.Commit},
		Tailnet:     tailnet.Status{State: catalog.StateUnknown},
	}
	var (
		wg                          sync.WaitGroup
		toolsErr, configErr         error
		sampleErr                   error
		sample                      Sample
		settingsHere                string
		hereOK                      bool
		sysFacts                    systemFacts
		identity, daemons, gateways []Check
		osName                      string
		daemon                      Check
	)
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	run(func() { snap.Tools, toolsErr = s.deps.Tools.Inventory(ctx) })
	run(func() { snap.Config, configErr = s.deps.Config.List(ctx) })
	run(func() {
		if st, err := s.deps.Tailnet.Status(ctx); err == nil {
			snap.Tailnet = st
		}
	})
	run(func() { snap.Setup = s.setup(ctx) })
	run(func() { sample, sampleErr = s.Sample(ctx) })
	run(func() { sysFacts = s.systemFacts(ctx) })
	run(func() { osName = s.osName(ctx) })
	run(func() { snap.Environment = s.environment(ctx) })
	run(func() { snap.Workspace = s.workspace(ctx) })
	run(func() { identity = s.identity(ctx) })
	run(func() { daemons = s.daemons(ctx) })
	run(func() { gateways = s.gateways(ctx) })
	run(func() { daemon = s.daemon(ctx) })
	wg.Wait()

	if current, err := s.deps.Store.Load(); err == nil && current.Here != "" {
		settingsHere, hereOK = current.Here, true
	}
	if toolsErr != nil {
		return Snapshot{}, toolsErr
	}
	if configErr != nil {
		return Snapshot{}, configErr
	}
	if sampleErr != nil {
		return Snapshot{}, sampleErr
	}
	snap.Machine = Machine{
		Name:       settingsHere,
		Registered: hereOK,
		Kind:       s.o.Here,
		Hostname:   s.h.Env.Hostname(),
		OS:         osName,
		Arch:       s.h.Env.GOARCH(),
		User:       s.h.Env.User(),
		Shell:      shellName(s.h.Env.Shell()),
	}
	snap.System = System{Uptime: sysFacts.uptime, Battery: sysFacts.battery, MemoryTotal: sysFacts.memory, Sample: sample}
	snap.Identity, snap.Daemons, snap.Gateways = identity, daemons, gateways
	snap.Station.Daemon = daemon
	if s.mac() {
		for _, d := range [][2]string{{"sshd", "remote-login"}, {"dns", "encrypted-dns"}} {
			if check, placed := fromConfig(snap.Config, d[0], d[1]); placed {
				snap.Environment.Security = append(snap.Environment.Security, check)
			}
		}
	}
	return snap, nil
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
		case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown:
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
	case err != nil:
		c.State, c.Value, c.Detail = catalog.StateDegraded, "not running", "station setup"
	case version != s.o.Version:
		c.State, c.Value, c.Detail = catalog.StateDegraded, "runs "+version+", station is "+s.o.Version, "station setup"
	default:
		c.State, c.Value = catalog.StateOK, "running "+version
	}
	return c
}
