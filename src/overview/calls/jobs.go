package calls

import (
	"context"
	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/ports"
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
				reading, err := activitycalls.Read(ctx, r.d.Meter)
				return func(s *sn) {
					if err != nil {
						s.Err = err
						return
					}
					s.System.Reading = reading
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
