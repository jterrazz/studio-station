package status

import (
	"context"
	"path"
	"slices"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	schedule "github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	subject    = "status"
	margin     = time.Second
	partBudget = 60 * time.Second
)

type job = schedule.Job[Snapshot]

func (s *Service) Watch(ctx context.Context) <-chan Snapshot {
	return schedule.Stream(ctx, s.o.Pool, subject, s.watch)
}

func (s *Service) Forget() {
	// invariant: a reload of the whole machine drops every shared reading of
	// the run (the inventory, the walk, the shared commands), not only its own.
	s.o.Pool.Forget("")
}

func pendingChecks(table []probe) []Check {
	checks := make([]Check, len(table))
	for i, p := range table {
		checks[i] = Check{Name: p.name, State: catalog.StatePending}
	}
	return checks
}

func budgetOf(p probe) time.Duration {
	if p.budget <= 0 {
		return defaultBudget
	}
	return p.budget
}

// invariant: the frame is what a read knows from settings alone: every section
// and every row a probe table declares, each pending; a job per row then fills
// it, so a slow tool delays only its own row.
func (s *Service) frame() Snapshot {
	snap := Snapshot{
		GeneratedAt: s.h.Clock.Now().UTC(),
		Station:     Station{Version: s.o.Version, Commit: s.o.Commit, Daemon: Check{Name: "daemon", State: catalog.StatePending}},
		Tailnet:     ports.Status{State: catalog.StatePending},
		Setup:       Setup{State: catalog.StatePending, Steps: []setup.Step{}},
		System:      System{Pending: true, Sample: Sample{CPU: []Process{}, Memory: []Process{}}},
		Identity:    append(pendingChecks(s.identityTable()), Check{Name: "ssh-key", State: catalog.StatePending}),
		Background:  []Job{},
		Gateways:    []Check{},
		Environment: Environment{Network: pendingChecks(s.networkProbes())},
		Workspace: Workspace{
			Repositories: []Repository{}, Dependencies: []Dependency{}, Docker: Docker{State: catalog.StatePending},
			Disk: []Disk{}, Strays: []storage.StrayReport{}, ReposPending: true, DepsPending: true, DiskPending: true,
		},
		ToolsPending:  true,
		ConfigPending: true,
	}
	if s.mac() {
		snap.Environment.Ports = []Port{}
		snap.Environment.Security = pendingChecks(s.securityProbes())
		snap.Environment.Updates = pendingChecks(s.updateProbes())
	}
	here, err := s.deps.Name.Get()
	snap.Machine = Machine{
		Name:       here,
		Registered: err == nil && here != "",
		Kind:       s.o.Here,
		Hostname:   s.h.Env.Hostname(),
		OS:         s.h.Env.GOOS(),
		Arch:       s.h.Env.GOARCH(),
		User:       s.h.Env.User(),
		Shell:      shellName(s.h.Env.Shell()),
	}
	if !snap.Machine.Registered {
		snap.Machine.Name = ""
	}
	return snap
}

func (s *Service) watch(ctx context.Context) <-chan Snapshot {
	snap := s.frame()
	jobs := s.machineJobs()
	jobs = append(jobs, s.tableJobs("environment/network", s.networkProbes(), func(sn *Snapshot) []Check { return sn.Environment.Network })...)
	if s.mac() {
		jobs = append(jobs, s.tableJobs("environment/security", s.securityProbes(), func(sn *Snapshot) []Check { return sn.Environment.Security })...)
		jobs = append(jobs, s.tableJobs("environment/updates", s.updateProbes(), func(sn *Snapshot) []Check { return sn.Environment.Updates })...)
		jobs = append(jobs, job{Key: subject + "/environment/ports", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				found := s.listening(ctx)
				return func(sn *Snapshot) { sn.Environment.Ports = found }
			},
			Late: func(*Snapshot) {},
		})
	}
	jobs = append(jobs, s.workspaceJobs()...)
	jobs = append(jobs, s.tableJobs("identity", s.identityTable(), func(sn *Snapshot) []Check { return sn.Identity })...)
	jobs = append(jobs, s.sectionJobs()...)
	return schedule.Run(ctx, s.o.Pool, snap, jobs, cloneSnapshot)
}

func cloneSnapshot(sn Snapshot) Snapshot {
	sn.Identity = slices.Clone(sn.Identity)
	sn.Environment.Network = slices.Clone(sn.Environment.Network)
	sn.Environment.Security = slices.Clone(sn.Environment.Security)
	sn.Environment.Updates = slices.Clone(sn.Environment.Updates)
	sn.Gateways = slices.Clone(sn.Gateways)
	return sn
}

func (s *Service) tableJobs(key string, table []probe, rows func(*Snapshot) []Check) []job {
	out := make([]job, 0, len(table))
	for i, p := range table {
		budget := budgetOf(p)
		out = append(out, job{
			Key: subject + "/" + key + "/" + p.name, Budget: budget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				check := s.probe(ctx, p)
				return func(sn *Snapshot) { rows(sn)[i] = check }
			},
			Late: func(sn *Snapshot) { rows(sn)[i] = late(p.name, budget) },
		})
	}
	return out
}

func (s *Service) machineJobs() []job {
	return []job{
		{Key: subject + "/machine/os", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				name := s.osName(ctx)
				return func(sn *Snapshot) { sn.Machine.OS = name }
			},
			Late: func(*Snapshot) {},
		},
		{Key: subject + "/machine/facts", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				facts := s.systemFacts(ctx)
				return func(sn *Snapshot) {
					sn.System.Uptime, sn.System.Battery, sn.System.MemoryTotal, sn.System.Pending = facts.uptime, facts.battery, facts.memory, false
				}
			},
			Late: func(sn *Snapshot) { sn.System.Pending = false },
		},
		{Key: subject + "/machine/sample", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				sample, err := s.Sample(ctx)
				return func(sn *Snapshot) {
					if err != nil {
						sn.err = err
						return
					}
					sn.System.Sample = sample
				}
			},
			Late: func(*Snapshot) {},
		},
		{Key: subject + "/machine/setup", Budget: partBudget,
			Do: func(ctx context.Context) func(*Snapshot) {
				st := s.setup(ctx)
				return func(sn *Snapshot) { sn.Setup = st }
			},
			Late: func(sn *Snapshot) { sn.Setup = Setup{State: catalog.StateUnknown, Steps: []setup.Step{}} },
		},
		{Key: subject + "/machine/daemon", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				d := s.daemon(ctx)
				return func(sn *Snapshot) { sn.Station.Daemon = d }
			},
			Late: func(sn *Snapshot) { sn.Station.Daemon = Check{Name: "daemon", State: catalog.StateAbsent} },
		},
		{Key: subject + "/machine/power", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				if s.o.Here != catalog.Host || s.deps.Power == nil {
					return func(*Snapshot) {}
				}
				p, ok := s.deps.Power.Session(ctx)
				return func(sn *Snapshot) { sn.Power, sn.PowerKnown = p, ok }
			},
			Late: func(*Snapshot) {},
		},
	}
}

func (s *Service) workspaceJobs() []job {
	root := path.Join(s.h.Env.Home(), "Developer")
	return []job{
		{Key: subject + "/workspace/repos", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				repos := s.repositories(ctx, root, s.findRepos(root, 0))
				return func(sn *Snapshot) { sn.Workspace.Repositories, sn.Workspace.ReposPending = repos, false }
			},
			Late: func(sn *Snapshot) { sn.Workspace.ReposPending = false },
		},
		{Key: subject + "/workspace/deps", Budget: scanBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				deps := s.dependencies(ctx, root, s.findRepos(root, 0))
				return func(sn *Snapshot) { sn.Workspace.Dependencies, sn.Workspace.DepsPending = deps, false }
			},
			Late: func(sn *Snapshot) { sn.Workspace.DepsPending = false },
		},
		{Key: subject + "/workspace/docker", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				d := s.docker(ctx)
				return func(sn *Snapshot) { sn.Workspace.Docker = d }
			},
			Late: func(sn *Snapshot) { sn.Workspace.Docker = Docker{State: catalog.StateUnknown} },
		},
		{Key: subject + "/workspace/disk", Lane: schedule.Inline,
			// invariant: the disk row follows the run's one storage reading, so its
			// frame shows the last-known total until the walk answers.
			Partial: func(ctx context.Context, emit func(func(*Snapshot))) func(*Snapshot) {
				var report storage.Report
				if s.deps.Storage != nil {
					for r := range s.deps.Storage.Watch(ctx) {
						report = r
						d, strays := disk(r)
						emit(func(sn *Snapshot) { sn.Workspace.Disk, sn.Workspace.Strays = d, strays })
					}
				}
				d, strays := disk(report)
				return func(sn *Snapshot) {
					sn.Workspace.Disk, sn.Workspace.Strays, sn.Workspace.DiskPending = d, strays, false
				}
			},
			Late: func(sn *Snapshot) { sn.Workspace.DiskPending = false },
		},
	}
}

func (s *Service) sectionJobs() []job {
	return []job{
		{Key: subject + "/identity/ssh-key", Budget: defaultBudget,
			Do: func(context.Context) func(*Snapshot) {
				check := s.sshKey()
				return func(sn *Snapshot) { sn.Identity[len(sn.Identity)-1] = check }
			},
			Late: func(sn *Snapshot) { sn.Identity[len(sn.Identity)-1] = late("ssh-key", defaultBudget) },
		},
		{Key: subject + "/background", Budget: 2*defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				jobs := s.background(ctx)
				return func(sn *Snapshot) { sn.Background = jobs }
			},
			Late: func(*Snapshot) {},
		},
		{Key: subject + "/gateways", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*Snapshot) {
				checks := s.gateways(ctx)
				return func(sn *Snapshot) { sn.found = checks; s.placeGateways(sn) }
			},
			Late: func(*Snapshot) {},
		},
		{Key: subject + "/tailnet", Budget: partBudget,
			Do: func(ctx context.Context) func(*Snapshot) {
				st, err := s.deps.Tailnet.Status(ctx)
				if err != nil {
					st = ports.Status{State: catalog.StateUnknown}
				}
				return func(sn *Snapshot) { sn.Tailnet = st; s.placeGateways(sn) }
			},
			Late: func(sn *Snapshot) { sn.Tailnet = ports.Status{State: catalog.StateUnknown} },
		},
		{Key: subject + "/tools", Lane: schedule.Inline,
			Do: func(ctx context.Context) func(*Snapshot) {
				reports, err := s.deps.Tools.Inventory(ctx)
				return func(sn *Snapshot) {
					sn.Tools, sn.ToolsPending = reports, false
					if err != nil {
						sn.err = err
					}
				}
			},
			Late: func(sn *Snapshot) { sn.ToolsPending = false },
		},
		{Key: subject + "/config", Budget: partBudget,
			Do: func(ctx context.Context) func(*Snapshot) {
				reports, err := s.deps.Config.List(ctx)
				return func(sn *Snapshot) {
					sn.Config, sn.ConfigPending = reports, false
					if err != nil {
						sn.err = err
						return
					}
					if s.mac() {
						for _, d := range [][2]string{{"sshd", "remote-login"}, {"dns", "encrypted-dns"}} {
							if check, placed := fromConfig(reports, d[0], d[1]); placed {
								sn.Environment.Security = append(sn.Environment.Security, check)
							}
						}
					}
				}
			},
			Late: func(sn *Snapshot) { sn.ConfigPending = false },
		},
	}
}

func (s *Service) placeGateways(sn *Snapshot) {
	sn.Gateways = slices.Clone(sn.found)
	if sn.Gateways == nil {
		sn.Gateways = []Check{}
	}
	if s.o.Here == catalog.Host && sn.Tailnet.Connected {
		sn.Gateways = append(sn.Gateways, serveGateway(sn.Tailnet.ServeOn))
	}
}

func (s *Service) updateProbes() []probe {
	return []probe{
		s.factProbe("macos", catalog.FactSystemUpdates, updateBudget),
		{name: "brew", cmd: cmd("brew", "outdated", "--quiet"), budget: updateBudget, interpret: brewOutdated},
	}
}
