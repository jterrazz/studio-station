package wire

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/engine"
	enginelog "github.com/jterrazz/studio-station/src/engine/adapter/log"
	"github.com/jterrazz/studio-station/src/engine/feeds"
	"github.com/jterrazz/studio-station/src/engine/process"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	"github.com/jterrazz/studio-station/src/remote"
	"github.com/jterrazz/studio-station/src/remote/api"
	daemoncmd "github.com/jterrazz/studio-station/src/surface/cli/daemon"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/system"
)

func (m machine) guestPort() int { return m.port(22, "STATION_GUEST_SSH_PORT") }

// invariant: inside the engine, the menu bar reads through this run's
// Router, so its fleet reaches the other Macs as the CLI's does; a recall
// rides the engine's own connections, never its socket: the lends are bound
// to the engine they serve.
func (m machine) daemon(env kit.Env) *engine.Daemon {
	lends := &daemonLends{}
	registry := m.registry(env)
	calls := env.Calls
	if calls == nil {
		calls = remote.NewRouter(registry, m.profile, remote.Reach{})
	}
	menubar := feeds.Menubar{Calls: calls, Clock: m.host.Clock, Here: m.where, Report: func(v power.View) *power.Report {
		return &power.Report{Now: m.host.Clock.Now(), Zone: time.Local, Daemon: power.DaemonRunning, Reading: v.Reading, Session: v.Session, LastEnd: v.LastEnd, Guards: v.Guards}
	}}
	guests := engineGuests{dialer: m.guestDialer(), vms: m.machines(false)}
	notifier := func(write func(subject, text string)) engine.Notifier { return enginelog.New(write) }
	var tailnet portscontract.Tailnet = m.tailscale().Tracked(engine.ChildrenDir(m.paths.State))
	if m.world != nil {
		notifier = func(func(subject, text string)) engine.Notifier { return m.world.notifier() }
		tailnet = m.world.tailnet()
	}
	d := engine.New(m.host, engineHypervisor{m.hypervisor()}, guests, m.machines(false), tailnet, engine.Stores{Services: m.stores.services(), Reservations: m.stores.reservations}, engine.Options{
		State: m.paths.State, Here: m.kind, Version: Version(), PID: os.Getpid(), GuestPort: m.guestPort(),
		Power: m.powerPort, PowerPage: m.powerPage, Executable: m.host.Env.Executable(),
		Lends: lends, Peer: m.engineAPI(),
		Feeds:    map[string]engine.Feed{engine.PageMenubar: menubar.Feed()},
		Notifier: notifier,
	})
	lends.s = m.secretsVia(d)
	d.Host(m.engineServer(d, registry))
	return d
}

func daemonCommand(env kit.Env, m machine) *cobra.Command {
	return daemoncmd.Command(daemoncmd.Deps{Daemon: m.daemon(env)}, env)
}

type daemonLends struct{ s *secrets.Service }

func (l *daemonLends) Held() (map[string][]engine.Lend, error) {
	held, err := l.s.Lends()
	out := make(map[string][]engine.Lend, len(held))
	for vm, list := range held {
		for _, x := range list {
			out[vm] = append(out[vm], engine.Lend{Name: x.Name, Until: x.Until})
		}
	}
	return out, err
}

func (l *daemonLends) Recall(ctx context.Context, from string, names []string) error {
	_, err := l.s.Recall(ctx, from, names)
	return err
}

type hostDaemon struct{ c engineAPI }

func (m machine) hostDaemon() hostDaemon { return hostDaemon{c: m.engineAPI()} }

func downAs(err, sentinel error) error {
	var down *engine.DownError
	var gone *api.Down
	if errors.As(err, &down) || errors.As(err, &gone) {
		return sentinel
	}
	return err
}

func (d hostDaemon) Ping(ctx context.Context) (string, error) {
	p, err := d.c.Hello(ctx)
	return p.Version, downAs(err, machines.ErrDaemonDown)
}

func (d hostDaemon) VMs(ctx context.Context) (map[string]machines.DaemonVM, error) {
	views, err := d.c.VMs(ctx)
	if err != nil {
		return nil, downAs(err, machines.ErrDaemonDown)
	}
	out := make(map[string]machines.DaemonVM, len(views))
	for _, v := range views {
		out[v.Name] = machines.DaemonVM{State: v.State, Guest: v.Guest, IP: v.IP, Detail: v.Detail}
	}
	return out, nil
}

func (d hostDaemon) GuestExec(ctx context.Context, vm, line, stdin string) (system.Output, error) {
	var ran process.ExecOut
	if err := d.c.do(ctx, process.GuestExec, process.ExecIn{VM: vm, Line: line, Stdin: stdin}, &ran); err != nil {
		return system.Output{}, downAs(err, machines.ErrDaemonDown)
	}
	return system.Output{Stdout: ran.Stdout, Stderr: ran.Stderr, ExitCode: ran.Code}, nil
}

func (d hostDaemon) Do(ctx context.Context, op, vm string) error {
	return downAs(d.c.do(ctx, process.VM(op), process.VMIn{VM: vm}, nil), machines.ErrDaemonDown)
}

func (d hostDaemon) Reserve(ctx context.Context, vm string, port int, profile string) (machines.Reserved, error) {
	var v services.View
	if err := d.c.do(ctx, process.Reserve, process.ReserveIn{VM: vm, GuestPort: port, Pinned: true, Profile: profile}, &v); err != nil {
		return machines.Reserved{}, downAs(err, machines.ErrDaemonDown)
	}
	return machines.Reserved{
		GuestPort: v.GuestPort, HostPort: v.HostPort, Profile: profile, URL: v.URLs.Public, PublicHost: hostOf(v.URLs.Public),
		ServeOff: strings.Contains(v.Detail, catalog.ServeOffDetail),
	}, nil
}

func hostOf(url string) string {
	_, rest, ok := strings.Cut(url, "://")
	if !ok {
		rest = url
	}
	host, _, _ := strings.Cut(rest, "/")
	host, _, _ = strings.Cut(host, ":")
	return host
}

type servicesDaemon struct{ c engineAPI }

func (d servicesDaemon) Services(ctx context.Context) (services.HostView, error) {
	var v services.HostView
	err := d.c.do(ctx, process.Services, nil, &v)
	return v, downAs(err, services.ErrDaemonDown)
}

func (d servicesDaemon) Reserve(ctx context.Context, vm string, port int, pinned bool) (services.View, error) {
	var v services.View
	err := d.c.do(ctx, process.Reserve, process.ReserveIn{VM: vm, GuestPort: port, Pinned: pinned}, &v)
	return v, downAs(err, services.ErrDaemonDown)
}

func (d servicesDaemon) Release(ctx context.Context, vm string, port int) (services.Lease, error) {
	var l services.Lease
	err := d.c.do(ctx, process.Release, process.ReserveIn{VM: vm, GuestPort: port}, &l)
	return l, downAs(err, services.ErrDaemonDown)
}

type leaseReleaser struct{ m machine }

func (r leaseReleaser) ReleaseVM(ctx context.Context, vm string) error {
	store := r.m.stores.reservations
	leases, err := store.List(ctx)
	if err != nil {
		return err
	}
	kept, gone := services.Without(leases, func(l services.Lease) bool { return l.VM != vm })
	if len(gone) == 0 {
		return nil
	}
	g, _ := r.m.stores.services().Get()
	mode := services.PolicyOf(g).HTTPS
	tn := r.m.tailnet()
	if ready, err := tn.Ready(ctx); err == nil && ready.On {
		for _, l := range gone {
			_ = tn.Unshare(ctx, services.ShareOf(l, mode))
		}
	}
	return services.SaveLeases(ctx, store, kept)
}

type daemonInstaller struct {
	m        machine
	machines *machines.Service
}

func (i daemonInstaller) env() map[string]string {
	env := map[string]string{"HOME": i.m.host.Env.Home()}
	for _, name := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "STATION_GUEST_SSH_PORT"} {
		if v := i.m.vars[name]; v != "" {
			env[name] = v
		}
	}
	return env
}

// invariant: setup installs ONE daemon per host, after migrating the round-3
// per-VM agents; it restarts the daemon only when its plist changed or it runs
// another version, and the VMs survive that restart.
func (i daemonInstaller) Install(ctx context.Context) (setup.Step, error) {
	const step = "daemon"
	if hv := i.m.hypervisor(); !hv.Installed() {
		return setup.Step{Name: step, State: catalog.StateAbsent, Detail: "not needed until " + hv.Name() + " is installed"}, nil
	}
	jobs := i.m.jobs(0)
	if !jobs.Present() {
		return setup.Step{Name: step, State: catalog.StateAbsent, Detail: jobs.Names().Manager + " is not here to run it"}, nil
	}
	if _, err := i.machines.MigrateAgents(ctx); err != nil {
		return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
	}
	job := i.m.daemonJob()
	changed, err := jobs.Install(ctx, contract.JobSpec{
		Label: job.Label, Program: []string{i.m.host.Env.Executable(), "daemon"}, Env: i.env(),
		AtLogin: true, KeepAlive: true, Log: filepath.Join(i.m.paths.State, "logs", "daemon.log"), File: job.File,
	})
	if err != nil {
		return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
	}
	version := Version()
	if !jobs.Read(ctx, job).Loaded {
		for _, run := range []func(context.Context, contract.Ref) error{jobs.Load, jobs.Start} {
			if runErr := run(ctx, job); runErr != nil {
				return setup.Step{Name: step, State: catalog.StateBroken, Detail: runErr.Error()}, &setup.StepFailedError{Step: step, Cause: runErr}
			}
		}
		return setup.Step{Name: step, State: catalog.StateOK, Detail: "running " + version, Changed: true}, nil
	}
	running, err := i.m.engineAPI().Hello(ctx)
	if changed || (err == nil && running.Version != version) {
		if kickErr := jobs.Restart(ctx, job); kickErr != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: kickErr.Error()}, &setup.StepFailedError{Step: step, Cause: kickErr}
		}
		detail := "restarted"
		if err == nil && running.Version != version {
			detail += " (" + running.Version + " to " + version + ")"
		}
		return setup.Step{Name: step, State: catalog.StateOK, Detail: detail, Changed: true}, nil
	}
	return setup.Step{Name: step, State: catalog.StateOK, Detail: "running " + version}, nil
}
