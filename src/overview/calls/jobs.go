package calls

import (
	"context"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	settingPowerPolicy = "power"
	settingAutoLogin   = "autologin"
)

func (r *Reader[Tool, Item, Stray]) machineJobs() []job[Tool, Item, Stray] {
	type sn = snapshot[Tool, Item, Stray]
	return []job[Tool, Item, Stray]{
		{Key: subject + "/machine/os", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				name := r.kernel(ctx)
				return func(s *sn) { s.Machine.OS = name }
			},
			Late: func(*sn) {},
		},
		{Key: subject + "/machine/facts", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				uptime, battery, memory := r.system(ctx)
				return func(s *sn) {
					s.System.Uptime, s.System.Battery, s.System.MemoryTotal, s.System.Pending = uptime, battery, memory, false
				}
			},
			Late: func(s *sn) { s.System.Pending = false },
		},
		{Key: subject + "/machine/sample", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				sample, err := r.d.Meter.Sample(ctx)
				return func(s *sn) {
					if err != nil {
						s.Err = err
						return
					}
					s.System.Sample = sample
				}
			},
			Late: func(*sn) {},
		},
		{Key: subject + "/machine/setup", Budget: partBudget,
			Do: func(ctx context.Context) func(*sn) {
				steps, err := r.d.Setup.Check(ctx)
				st := overview.SetupOf(steps)
				if err != nil {
					st = overview.Setup{State: catalog.StateUnknown, Steps: []overview.Step{}}
				}
				return func(s *sn) { s.Setup = st }
			},
			Late: func(s *sn) { s.Setup = overview.Setup{State: catalog.StateUnknown, Steps: []overview.Step{}} },
		},
		{Key: subject + "/machine/daemon", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				d := r.daemon(ctx)
				return func(s *sn) { s.Station.Daemon = d }
			},
			Late: func(s *sn) { s.Station.Daemon = overview.Check{Name: "daemon", State: catalog.StateAbsent} },
		},
		{Key: subject + "/machine/power", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				if r.d.Here.Kind != catalog.Host || r.d.Power == nil {
					return func(*sn) {}
				}
				p, ok := r.d.Power.Session(ctx)
				return func(s *sn) { s.Power, s.PowerKnown = p, ok }
			},
			Late: func(*sn) {},
		},
	}
}

// invariant: the readiness reads the settings first, and the Mac's facts
// only when the Mac needs a readiness at all.
func (r *Reader[Tool, Item, Stray]) readinessJob() job[Tool, Item, Stray] {
	type sn = snapshot[Tool, Item, Stray]
	return job[Tool, Item, Stray]{Key: subject + "/readiness", Budget: partBudget,
		Do: func(ctx context.Context) func(*sn) {
			if r.d.Here.Kind != catalog.Host {
				return func(*sn) {}
			}
			checks := r.readiness(ctx)
			return func(s *sn) { s.Readiness = checks }
		},
		Late: func(*sn) {},
	}
}

func (r *Reader[Tool, Item, Stray]) readiness(ctx context.Context) []overview.Check {
	states := map[string]catalog.State{}
	if items, err := r.d.Config.List(ctx); err == nil {
		for _, it := range items {
			name, state := r.d.Config.Placed(it)
			states[name] = state
		}
	}
	on := func(name string) machines.ReadinessSetting {
		state, ok := states[name]
		if !ok {
			state = catalog.StateUnknown
		}
		return machines.ReadinessSetting{Name: name, State: state, Root: needsRoot(name)}
	}
	f := machines.ReadinessFacts{
		Mac:                   r.label(),
		RestartsAfterPowerCut: on(settingPowerPolicy), NeverSleeps: on(settingPowerPolicy), LogsIn: on(settingAutoLogin),
		EngineKnown: r.d.Here.Agent != nil,
	}
	if machines.Readiness(f) == nil {
		return []overview.Check{}
	}
	f.FileVault, f.AuthRestart = r.factState(ctx, catalog.FactFileVault), r.factState(ctx, catalog.FactAuthRestart)
	f.EngineAtLogin = f.EngineKnown && r.d.Here.Agent()
	items := machines.Readiness(f)
	out := make([]overview.Check, 0, len(items))
	for _, it := range items {
		out = append(out, overview.Check{Name: it.Name, State: it.State, Value: it.Value, Detail: it.Fix})
	}
	return out
}

func needsRoot(setting string) bool {
	i := slices.IndexFunc(catalog.ConfigItems, func(it catalog.ConfigItem) bool { return it.Name == setting })
	return i >= 0 && catalog.ConfigItems[i].NeedsRoot
}

func (r *Reader[Tool, Item, Stray]) factState(ctx context.Context, f catalog.Fact) catalog.State {
	ctx, cancel := context.WithTimeout(ctx, defaultBudget)
	defer cancel()
	got, err := r.d.Facts.Read(ctx, f)
	if err != nil || ctx.Err() != nil {
		return catalog.StateUnknown
	}
	return got.State
}

func (r *Reader[Tool, Item, Stray]) label() string {
	name, _ := r.d.Name.Get()
	if short, ok := strings.CutPrefix(name, r.d.Here.User+"-"); ok && short != "" && r.d.Here.User != "" {
		return short
	}
	return name
}

func (r *Reader[Tool, Item, Stray]) diskJob() job[Tool, Item, Stray] {
	type sn = snapshot[Tool, Item, Stray]
	return job[Tool, Item, Stray]{Key: subject + "/workspace/disk", Lane: lanes.Inline,
		// invariant: the disk row follows the run's one storage reading, so its
		// frame shows the last-known total until the walk answers.
		Partial: func(ctx context.Context, emit func(func(*sn))) func(*sn) {
			last := DiskReading[Stray]{Rows: []overview.Disk{}, Strays: []Stray{}}
			if r.d.Disk != nil {
				for d := range r.d.Disk.Watch(ctx) {
					last = d
					emit(func(s *sn) { s.Workspace.Disk, s.Workspace.Strays = d.Rows, d.Strays })
				}
			}
			return func(s *sn) {
				s.Workspace.Disk, s.Workspace.Strays, s.Workspace.DiskPending = last.Rows, last.Strays, false
			}
		},
		Late: func(s *sn) { s.Workspace.DiskPending = false },
	}
}

func (r *Reader[Tool, Item, Stray]) sectionJobs() []job[Tool, Item, Stray] {
	type sn = snapshot[Tool, Item, Stray]
	return []job[Tool, Item, Stray]{
		{Key: subject + "/background", Budget: 2*defaultBudget + margin,
			Do: func(ctx context.Context) func(*sn) {
				jobs := []background.Task{}
				if r.mac() && r.d.Jobs != nil {
					listed, _ := r.d.Jobs.List(ctx)
					jobs = background.Judge(listed, r.d.Here.Root, r.d.Here.Home)
				}
				return func(s *sn) { s.Background = jobs }
			},
			Late: func(*sn) {},
		},
		{Key: subject + "/tailnet", Budget: partBudget,
			Do: func(ctx context.Context) func(*sn) {
				st, err := r.d.Tailnet.Status(ctx)
				if err != nil {
					st = ports.Status{State: catalog.StateUnknown}
				}
				return func(s *sn) { s.Tailnet = st; s.Gateways = overview.Gateways(r.d.Here.Kind, st) }
			},
			Late: func(s *sn) { s.Tailnet = ports.Status{State: catalog.StateUnknown} },
		},
		{Key: subject + "/tools", Lane: lanes.Inline,
			Do: func(ctx context.Context) func(*sn) {
				reports, err := r.d.Tools.Inventory(ctx)
				return func(s *sn) {
					s.Tools, s.ToolsPending = reports, false
					if err != nil {
						s.Err = err
					}
				}
			},
			Late: func(s *sn) { s.ToolsPending = false },
		},
		{Key: subject + "/config", Budget: partBudget,
			Do: func(ctx context.Context) func(*sn) {
				reports, err := r.d.Config.List(ctx)
				return func(s *sn) {
					s.Config, s.ConfigPending = reports, false
					if err != nil {
						s.Err = err
						return
					}
					if r.mac() {
						s.Environment.Protections = append(s.Environment.Protections, r.protections(reports)...)
					}
				}
			},
			Late: func(s *sn) { s.ConfigPending = false },
		},
	}
}

func (r *Reader[Tool, Item, Stray]) protections(reports []Item) []overview.Check {
	var out []overview.Check
	for _, p := range overview.Protections() {
		for _, rep := range reports {
			if item, state := r.d.Config.Placed(rep); item == p[0] {
				out = append(out, overview.Protection(state, p[1]))
				break
			}
		}
	}
	return out
}
