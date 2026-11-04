package calls

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	appscontract "github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/settings"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
)

const (
	subject       = "status"
	defaultBudget = 2 * time.Second
	updateBudget  = 10 * time.Second
	ghBudget      = 5 * time.Second
	margin        = time.Second
	partBudget    = 60 * time.Second
)

type Reader[Tool, Item, Stray any] struct{ d Deps[Tool, Item, Stray] }

type snapshot[Tool, Item, Stray any] = overview.Snapshot[Tool, Item, Stray]

func New[Tool, Item, Stray any](d Deps[Tool, Item, Stray]) *Reader[Tool, Item, Stray] {
	return &Reader[Tool, Item, Stray]{d: d}
}

func (r *Reader[Tool, Item, Stray]) mac() bool { return r.d.Here.Kind != catalog.Linux }

func (r *Reader[Tool, Item, Stray]) Watch(ctx context.Context) <-chan snapshot[Tool, Item, Stray] {
	return lanes.Stream(ctx, r.d.Pool, subject, r.watch)
}

func (r *Reader[Tool, Item, Stray]) Snapshot(ctx context.Context) (snapshot[Tool, Item, Stray], error) {
	snap := lanes.Last(r.Watch(ctx))
	if err := ctx.Err(); err != nil {
		return snapshot[Tool, Item, Stray]{}, err
	}
	return snap, snap.Err
}

func (r *Reader[Tool, Item, Stray]) Sample(ctx context.Context) (activity.Sample, error) {
	return r.d.Meter.Sample(ctx)
}

func (r *Reader[Tool, Item, Stray]) Forget() {
	// invariant: a reload of the whole machine drops every shared reading of
	// the run (the inventory, the walk, the shared commands), not only its own.
	r.d.Pool.Forget("")
}

type row struct {
	name   string
	budget time.Duration
	read   func(ctx context.Context) (overview.Check, error)
}

func pendingChecks(table []row) []overview.Check {
	checks := make([]overview.Check, len(table))
	for i, p := range table {
		checks[i] = overview.Check{Name: p.name, State: catalog.StatePending}
	}
	return checks
}

// invariant: a row past its budget answers late, whatever its tool says.
func fact(i settingscontract.Inspector, name string, f catalog.Fact, budget time.Duration, judge func(settings.Fact) overview.Check) row {
	if judge == nil {
		judge = func(got settings.Fact) overview.Check { return overview.FactRow(name, got) }
	}
	return row{name: name, budget: budget, read: func(ctx context.Context) (overview.Check, error) {
		got, err := i.Read(ctx, f)
		if err != nil {
			return overview.Check{}, err
		}
		return judge(got), nil
	}}
}

func (r *Reader[Tool, Item, Stray]) networkRows() []row {
	return []row{
		fact(r.d.Facts, "local ip", catalog.FactLocalIP, defaultBudget, nil),
		fact(r.d.Facts, "dns", catalog.FactDNS, defaultBudget, nil),
	}
}

func (r *Reader[Tool, Item, Stray]) protectionRows() []row {
	return []row{
		fact(r.d.Facts, "filevault", catalog.FactFileVault, defaultBudget, nil),
		fact(r.d.Facts, "firewall", catalog.FactFirewall, defaultBudget, nil),
		fact(r.d.Facts, "sip", catalog.FactSIP, defaultBudget, nil),
		fact(r.d.Facts, "gatekeeper", catalog.FactGatekeeper, defaultBudget, nil),
	}
}

func (r *Reader[Tool, Item, Stray]) updateRows() []row {
	rows := []row{fact(r.d.Facts, "macos", catalog.FactSystemUpdates, updateBudget, nil)}
	if u := r.d.Updates; u != nil {
		rows = append(rows, row{name: u.Manager(), budget: updateBudget, read: func(ctx context.Context) (overview.Check, error) {
			n, err := u.Behind(ctx)
			if ctx.Err() != nil {
				return overview.Check{}, ctx.Err()
			}
			return overview.AppUpdates(u.Manager(), n, errors.Is(err, appscontract.ErrMissing), err != nil), nil
		}})
	}
	return rows
}

func (r *Reader[Tool, Item, Stray]) identityRows() []row {
	want, _ := r.d.Identity.Get()
	git := func(name string, expected string) func(settings.Fact) overview.Check {
		return func(got settings.Fact) overview.Check { return overview.GitValue(name, got, expected) }
	}
	return []row{
		fact(r.d.Owner, "git-email", catalog.FactGitEmail, defaultBudget, git("git-email", want.Email())),
		fact(r.d.Owner, "git-name", catalog.FactGitName, defaultBudget, git("git-name", want.Name())),
		fact(r.d.Owner, "git-signing", catalog.FactGitSigning, defaultBudget, nil),
		fact(r.d.Owner, "gpg-key", catalog.FactGPGKey, defaultBudget, nil),
		fact(r.d.Owner, "github", catalog.FactGitHub, ghBudget, nil),
		fact(r.d.Owner, "ssh-key", catalog.FactSSHKey, defaultBudget, nil),
	}
}

// invariant: the frame is what a read knows from preferences alone: every
// section and every row a table declares, each pending; a job per row then
// fills it, so a slow tool delays only its own row.
func (r *Reader[Tool, Item, Stray]) frame() snapshot[Tool, Item, Stray] {
	h := r.d.Here
	snap := snapshot[Tool, Item, Stray]{
		GeneratedAt:   r.d.Clock.Now().UTC(),
		Station:       overview.Station{Version: h.Version, Commit: h.Commit, Daemon: overview.Check{Name: "daemon", State: catalog.StatePending}},
		Tailnet:       ports.Status{State: catalog.StatePending},
		Setup:         overview.Setup{State: catalog.StatePending, Steps: []overview.Step{}},
		System:        overview.System{Pending: true, Sample: activity.Sample{CPU: []activity.Process{}, Memory: []activity.Process{}}},
		Identity:      pendingChecks(r.identityRows()),
		Background:    []background.Task{},
		Gateways:      []overview.Check{},
		Environment:   overview.Environment{Network: pendingChecks(r.networkRows())},
		Workspace:     overview.Workspace[Stray]{Disk: []overview.Disk{}, Strays: []Stray{}, DiskPending: true},
		ToolsPending:  true,
		ConfigPending: true,
	}
	if r.mac() {
		snap.Environment.Ports = []overview.Port{}
		snap.Environment.Protections = pendingChecks(r.protectionRows())
		snap.Environment.Updates = pendingChecks(r.updateRows())
	}
	here, err := r.d.Name.Get()
	snap.Machine = overview.Machine{
		Name: here, Registered: err == nil && here != "", Kind: h.Kind,
		Hostname: h.Hostname, OS: h.OS, Arch: h.Arch, User: h.User, Shell: overview.ShellName(h.Shell),
	}
	if !snap.Machine.Registered {
		snap.Machine.Name = ""
	}
	return snap
}

type job[Tool, Item, Stray any] = lanes.Job[snapshot[Tool, Item, Stray]]

func (r *Reader[Tool, Item, Stray]) watch(ctx context.Context) <-chan snapshot[Tool, Item, Stray] {
	jobs := r.machineJobs()
	jobs = append(jobs, r.rowJobs("environment/network", r.networkRows(), func(sn *snapshot[Tool, Item, Stray]) []overview.Check { return sn.Environment.Network })...)
	if r.mac() {
		jobs = append(jobs, r.rowJobs("environment/protections", r.protectionRows(), func(sn *snapshot[Tool, Item, Stray]) []overview.Check { return sn.Environment.Protections })...)
		jobs = append(jobs, r.rowJobs("environment/updates", r.updateRows(), func(sn *snapshot[Tool, Item, Stray]) []overview.Check { return sn.Environment.Updates })...)
		jobs = append(jobs, job[Tool, Item, Stray]{Key: subject + "/environment/ports", Budget: defaultBudget + margin,
			Do: func(ctx context.Context) func(*snapshot[Tool, Item, Stray]) {
				found := r.listening(ctx)
				return func(sn *snapshot[Tool, Item, Stray]) { sn.Environment.Ports = found }
			},
			Late: func(*snapshot[Tool, Item, Stray]) {},
		})
	}
	jobs = append(jobs, r.diskJob())
	jobs = append(jobs, r.rowJobs("identity", r.identityRows(), func(sn *snapshot[Tool, Item, Stray]) []overview.Check { return sn.Identity })...)
	jobs = append(jobs, r.sectionJobs()...)
	jobs = append(jobs, r.readinessJob())
	return lanes.Run(ctx, r.d.Pool, r.frame(), jobs, clone[Tool, Item, Stray])
}

func clone[Tool, Item, Stray any](sn snapshot[Tool, Item, Stray]) snapshot[Tool, Item, Stray] {
	sn.Identity = slices.Clone(sn.Identity)
	sn.Environment.Network = slices.Clone(sn.Environment.Network)
	sn.Environment.Protections = slices.Clone(sn.Environment.Protections)
	sn.Environment.Updates = slices.Clone(sn.Environment.Updates)
	sn.Gateways = slices.Clone(sn.Gateways)
	sn.Readiness = slices.Clone(sn.Readiness)
	return sn
}

func (r *Reader[Tool, Item, Stray]) rowJobs(key string, table []row, rows func(*snapshot[Tool, Item, Stray]) []overview.Check) []job[Tool, Item, Stray] {
	out := make([]job[Tool, Item, Stray], 0, len(table))
	for i, p := range table {
		out = append(out, job[Tool, Item, Stray]{
			Key: subject + "/" + key + "/" + p.name, Budget: p.budget + margin,
			Do: func(ctx context.Context) func(*snapshot[Tool, Item, Stray]) {
				check := read(ctx, p)
				return func(sn *snapshot[Tool, Item, Stray]) { rows(sn)[i] = check }
			},
			Late: func(sn *snapshot[Tool, Item, Stray]) { rows(sn)[i] = overview.Late(p.name, p.budget) },
		})
	}
	return out
}

func read(ctx context.Context, p row) overview.Check {
	ctx, cancel := context.WithTimeout(ctx, p.budget)
	defer cancel()
	check, err := p.read(ctx)
	if err != nil || ctx.Err() != nil {
		return overview.Late(p.name, p.budget)
	}
	return check
}

func (r *Reader[Tool, Item, Stray]) listening(ctx context.Context) []overview.Port {
	if r.d.Listeners == nil {
		return []overview.Port{}
	}
	ctx, cancel := context.WithTimeout(ctx, defaultBudget)
	defer cancel()
	ls, err := r.d.Listeners.Read(ctx)
	if err != nil || ctx.Err() != nil {
		return []overview.Port{}
	}
	return overview.Listening(ls)
}

func (r *Reader[Tool, Item, Stray]) kernel(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, defaultBudget)
	defer cancel()
	if got, err := r.d.Facts.Read(ctx, catalog.FactKernel); err == nil && got.State == catalog.StateOK && got.Value != "" {
		return got.Value
	}
	return r.d.Here.OS
}

func (r *Reader[Tool, Item, Stray]) system(ctx context.Context) (uptime, battery string, memory int64) {
	ctx, cancel := context.WithTimeout(ctx, defaultBudget)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		if up, ok := r.d.Meter.Uptime(ctx); ok {
			uptime = activity.Uptime(up)
		}
	}()
	go func() {
		defer wg.Done()
		battery = r.d.Meter.Battery(ctx)
	}()
	go func() {
		defer wg.Done()
		memory = r.d.Meter.Memory(ctx)
	}()
	wg.Wait()
	return uptime, battery, memory
}

func (r *Reader[Tool, Item, Stray]) daemon(ctx context.Context) overview.Check {
	absent := overview.Check{Name: "daemon", State: catalog.StateAbsent}
	if r.d.Here.Kind != catalog.Host || r.d.Daemon == nil {
		return absent
	}
	d := overview.Daemon{Hypervisor: r.d.Hypervisor.Name(), Installed: r.d.Hypervisor.Installed()}
	if d.Installed {
		d.Version, d.Err = r.d.Daemon.Ping(ctx)
		d.Agent = d.Err != nil && r.d.Here.Agent != nil && r.d.Here.Agent()
	}
	return overview.DaemonRow(d, r.d.Here.Version)
}
