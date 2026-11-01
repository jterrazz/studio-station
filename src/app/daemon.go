package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	daemoncmd "github.com/jterrazz/studio-station/src/cli/daemon"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/daemon"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/sshclient"
	"github.com/jterrazz/studio-station/src/system/tart"
)

func (m machine) guestPort() int { return m.port(22, "STATION_GUEST_SSH_PORT") }

func (m machine) daemon() *daemon.Daemon {
	return daemon.New(m.host, tart.New(m.host.Run), sshclient.New(), m.machines(false), m.tailnet().Tracked(daemon.ChildrenDir(m.paths)), m.store, daemon.Options{
		Paths: m.paths, Here: m.kind, Version: Version(), PID: os.Getpid(), GuestPort: m.guestPort(),
	})
}

func daemonCommand(env kit.Env, m machine) *cobra.Command {
	return daemoncmd.Command(daemoncmd.Deps{Daemon: m.daemon()}, env)
}

type hostDaemon struct{ c daemon.Client }

func (m machine) hostDaemon() hostDaemon { return hostDaemon{c: daemon.Dial(m.host, m.paths)} }

func downAs(err, sentinel error) error {
	var down *daemon.DownError
	if errors.As(err, &down) {
		return sentinel
	}
	return err
}

func (d hostDaemon) Ping(ctx context.Context) (string, error) {
	p, err := d.c.Ping(ctx)
	return p.Version, downAs(err, machines.ErrDaemonDown)
}

func (d hostDaemon) VMs(ctx context.Context) (map[string]machines.DaemonVM, error) {
	var views []daemon.VMView
	if err := d.c.Do(ctx, "vms", daemon.Request{}, &views); err != nil {
		return nil, downAs(err, machines.ErrDaemonDown)
	}
	out := make(map[string]machines.DaemonVM, len(views))
	for _, v := range views {
		out[v.Name] = machines.DaemonVM{State: v.State, Guest: v.Guest, IP: v.IP, Detail: v.Detail}
	}
	return out, nil
}

func (d hostDaemon) Do(ctx context.Context, op, vm string) error {
	return downAs(d.c.Do(ctx, op, daemon.Request{VM: vm}, nil), machines.ErrDaemonDown)
}

func (d hostDaemon) Reserve(ctx context.Context, vm string, port int, profile string) (machines.Reserved, error) {
	var v services.View
	if err := d.c.Do(ctx, "reserve", daemon.Request{VM: vm, GuestPort: port, Pinned: true, Profile: profile}, &v); err != nil {
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

type servicesDaemon struct{ c daemon.Client }

func (d servicesDaemon) Services(ctx context.Context) (services.HostView, error) {
	var v services.HostView
	err := d.c.Do(ctx, "services", daemon.Request{}, &v)
	return v, downAs(err, services.ErrDaemonDown)
}

func (d servicesDaemon) Reserve(ctx context.Context, vm string, port int, pinned bool) (services.View, error) {
	var v services.View
	err := d.c.Do(ctx, "reserve", daemon.Request{VM: vm, GuestPort: port, Pinned: pinned}, &v)
	return v, downAs(err, services.ErrDaemonDown)
}

func (d servicesDaemon) Release(ctx context.Context, vm string, port int) (services.Lease, error) {
	var l services.Lease
	err := d.c.Do(ctx, "release", daemon.Request{VM: vm, GuestPort: port}, &l)
	return l, downAs(err, services.ErrDaemonDown)
}

type leaseReleaser struct{ m machine }

func (r leaseReleaser) ReleaseVM(ctx context.Context, vm string) error {
	store := services.NewStore(r.m.host.FS, r.m.paths.State)
	leases, err := store.Load()
	if err != nil {
		return err
	}
	kept, gone := services.Without(leases, func(l services.Lease) bool { return l.VM != vm })
	if len(gone) == 0 {
		return nil
	}
	cfg, _ := r.m.store.Load()
	mode := services.PolicyOf(cfg.Services).HTTPS
	tn := r.m.tailnet()
	if self, err := tn.Self(ctx); err == nil && self.ServeOn {
		for _, l := range gone {
			_ = tn.Serve(ctx, services.ServeArgs(l, mode, false)...)
		}
	}
	return store.Save(kept)
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

func (i daemonInstaller) launchctl(ctx context.Context, args ...string) (system.Output, error) {
	return i.m.host.Run.Output(ctx, system.Cmd{Name: "launchctl", Args: args})
}

// invariant: a launchctl call that exits non-zero failed: a daemon launchd never
// loaded or never started is no "running" row.
func (i daemonInstaller) mustLaunchctl(ctx context.Context, args ...string) error {
	out, err := i.launchctl(ctx, args...)
	if err != nil {
		return err
	}
	if out.ExitCode == 0 {
		return nil
	}
	message := "launchctl " + args[0] + " exited " + strconv.Itoa(out.ExitCode)
	if first, _, _ := strings.Cut(strings.TrimSpace(out.Stderr), "\n"); first != "" {
		message += ": " + first
	}
	return errors.New(message)
}

// invariant: setup installs ONE daemon per host, after migrating the round-3
// per-VM agents; it restarts the daemon only when its plist changed or it runs
// another version, and the VMs survive that restart.
func (i daemonInstaller) Install(ctx context.Context) (setup.Step, error) {
	const step = "daemon"
	if _, ok := i.m.host.Run.LookPath("tart"); !ok {
		return setup.Step{Name: step, State: catalog.StateAbsent, Detail: "not needed until tart is installed"}, nil
	}
	if _, ok := i.m.host.Run.LookPath("launchctl"); !ok {
		return setup.Step{Name: step, State: catalog.StateAbsent, Detail: "launchd is not here to run it"}, nil
	}
	if _, err := i.machines.MigrateAgents(ctx); err != nil {
		return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
	}
	home := i.m.host.Env.Home()
	file := filepath.Join(home, "Library", "LaunchAgents", daemon.Label+".plist")
	log := filepath.Join(i.m.paths.State, "logs", "daemon.log")
	plist := daemon.Plist(i.m.host.Env.Executable(), i.env(), log)
	current, _ := i.m.host.FS.ReadFile(file)
	changed := string(current) != plist
	for _, dir := range []string{filepath.Dir(file), filepath.Dir(log)} {
		if err := i.m.host.FS.MkdirAll(dir, 0o700); err != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
		}
	}
	if changed {
		if err := i.m.host.FS.WriteFile(file, []byte(plist), 0o644); err != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
		}
	}
	domain := "gui/" + strconv.Itoa(i.m.host.Env.Euid())
	target := domain + "/" + daemon.Label
	version := Version()
	if out, err := i.launchctl(ctx, "print", target); err != nil || out.ExitCode != 0 {
		if err := i.mustLaunchctl(ctx, "bootstrap", domain, file); err != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
		}
		if err := i.mustLaunchctl(ctx, "kickstart", target); err != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: err.Error()}, &setup.StepFailedError{Step: step, Cause: err}
		}
		return setup.Step{Name: step, State: catalog.StateOK, Detail: "running " + version, Changed: true}, nil
	}
	running, err := i.m.hostDaemon().c.Ping(ctx)
	if changed || (err == nil && running.Version != version) {
		if kickErr := i.mustLaunchctl(ctx, "kickstart", "-k", target); kickErr != nil {
			return setup.Step{Name: step, State: catalog.StateBroken, Detail: kickErr.Error()}, &setup.StepFailedError{Step: step, Cause: kickErr}
		}
		detail := "restarted"
		if err == nil && running.Version != version {
			detail += " (" + running.Version + " → " + version + ")"
		}
		return setup.Step{Name: step, State: catalog.StateOK, Detail: detail, Changed: true}, nil
	}
	return setup.Step{Name: step, State: catalog.StateOK, Detail: "running " + version}, nil
}
