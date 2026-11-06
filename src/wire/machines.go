package wire

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/adapter/macos"
	"github.com/jterrazz/studio-station/src/machines/adapter/openssh"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/machines/contract"
	settingsmacos "github.com/jterrazz/studio-station/src/settings/adapter/macos"
	settingscontract "github.com/jterrazz/studio-station/src/settings/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	machinescmd "github.com/jterrazz/studio-station/src/surface/cli/machines"
	machinesadd "github.com/jterrazz/studio-station/src/surface/cli/machines/add"
	machinescreate "github.com/jterrazz/studio-station/src/surface/cli/machines/create"
	machinesdelete "github.com/jterrazz/studio-station/src/surface/cli/machines/delete"
	machinesedit "github.com/jterrazz/studio-station/src/surface/cli/machines/edit"
	machinesforget "github.com/jterrazz/studio-station/src/surface/cli/machines/forget"
	machinespipe "github.com/jterrazz/studio-station/src/surface/cli/machines/pipe"
	machinesprobe "github.com/jterrazz/studio-station/src/surface/cli/machines/probe"
	machinesrestart "github.com/jterrazz/studio-station/src/surface/cli/machines/restart"
	machinesstart "github.com/jterrazz/studio-station/src/surface/cli/machines/start"
	machinesstop "github.com/jterrazz/studio-station/src/surface/cli/machines/stop"
	machinessupervise "github.com/jterrazz/studio-station/src/surface/cli/machines/supervise"
	machinesunlock "github.com/jterrazz/studio-station/src/surface/cli/machines/unlock"
	overviewcmd "github.com/jterrazz/studio-station/src/surface/cli/overview"
)

const (
	lanBudget           = 2 * time.Second
	defaultOpenClawPort = 18789
	defaultHermesPort   = 8642
)

func (m machine) machines(_ bool) *machines.Service {
	knownHosts := filepath.Join(m.paths.State, "known_hosts")
	return machines.New(m.host, m.hypervisor(), machines.Prefs{Name: m.stores.name(), Hosts: m.stores.hosts(), VMUser: m.stores.vmUser()}, m.stores.machines, hostNodes{tailnet: m.tailnet()}, allMachinesImages{service: m.images()}, m.hostEngine(), leaseReleaser{m: m}, machines.Options{
		Ports: machines.Ports{
			OpenClaw: m.port(defaultOpenClawPort, "OPENCLAW_GATEWAY_PORT"),
			Hermes:   m.port(defaultHermesPort, "HERMES_GATEWAY_PORT", "API_SERVER_PORT"),
		},
		Here:       m.kind,
		KnownHosts: knownHosts, Logs: filepath.Join(m.paths.State, "logs"),
		User:      m.host.Env.User(),
		Version:   m.stationVersion(),
		Ask:       m.askAllMachines,
		GuestPort: m.guestPort(),
		Jobs:      m.jobs(0),
		Unlocker:  m.unlocker(knownHosts),
		Answers:   m.answers,
		SSHConfig: m.sshConfig(knownHosts),
		Memory:    m.memory,
		LAN:       m.lanAddress,
	})
}

// value: this Mac's address on your LAN, as it reads it (its world's in
// the spec build).
func (m machine) lanAddress(ctx context.Context) string {
	var facts settingscontract.Inspector = settingsmacos.New(m.host.Run, m.host.FS, m.host.Clock, settingsmacos.Options{Root: m.systemRoot()})
	if m.world != nil {
		facts = m.world.inspector()
	}
	ctx, cancel := context.WithTimeout(ctx, lanBudget)
	defer cancel()
	got, err := facts.Read(ctx, catalog.FactLocalIP)
	if err != nil || got.State != catalog.StateOK {
		return ""
	}
	return got.Value
}

// value: the memory this Mac has, as it reads itself (its world's in the
// spec build).
func (m machine) memory(ctx context.Context) int64 {
	self := selfOf(m.host)
	if m.world != nil {
		self = m.world.self()
	}
	_, _, _, hw, _ := self.Read(ctx)
	return hw.Memory
}

// invariant: a Mac answers when its station does, on a connection dialed
// for the question: one kept from before it went down proves nothing.
func (m machine) answers(ctx context.Context, mac string) bool {
	r := m.far.router
	if r == nil {
		return false
	}
	r.Drop(mac)
	_, err := r.Hello(ctx, mac)
	return err == nil
}

func (m machine) unlocker(knownHosts string) contract.Unlocker {
	if m.world != nil {
		return m.world.unlocker()
	}
	return macos.New(m.host.Run, m.host.FS, knownHosts)
}

func (m machine) sshConfig(knownHosts string) contract.SSHConfig {
	if m.world != nil {
		return m.world.sshConfig()
	}
	return openssh.New(m.host.FS, openssh.Options{
		Home: m.host.Env.Home(), KnownHosts: knownHosts, Executable: m.host.Env.Executable(),
	})
}

func (m machine) port(fallback int, names ...string) int {
	for _, name := range names {
		if n, err := strconv.Atoi(m.vars[name]); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func machinesCommand(env kit.Env, m machine) *cobra.Command {
	service := m.machines(env.Interactive)
	overview := func(ctx context.Context, io kit.IO) error {
		return overviewcmd.Run(ctx, m.overviewDeps(false), env, false, io)
	}
	return machinescmd.Command(machinescmd.Deps{Here: m.where, Names: service.Names, Overview: overview}, env,
		machinescreate.Command(machinescreate.Deps{
			Resources: machines.ParseResources, Check: createCheck(service),
			NotBuilt: func(image string) error { return &machines.NoImageError{Image: image} },
			Admit: func() error {
				return call.Gate(call.Meta{Name: machinescalls.CreateWorkspace.Name, Applies: machinescalls.CreateWorkspace.Applies}, m.profile)
			},
			Fleet: func(ctx context.Context) (machines.Fleet, error) { return service.Fleet(ctx, false) },
			Here:  m.where, Ask: createAsker(env),
		}, env),
		machinesstart.Command(machinesstart.Deps{Names: service.Names, Here: m.where}, env),
		machinesstop.Command(machinesstop.Deps{Names: service.Names, Here: m.where}, env),
		machinesrestart.Command(machinesrestart.Deps{Names: service.Names, IsMac: func(name string) bool {
			r, err := service.Remote(name)
			return err == nil && r.Kind == catalog.Host && !env.Far()
		}, Pausing: m.pausing(env), Here: m.where}, env),
		machinesdelete.Command(machinesdelete.Deps{Names: service.Names, Here: m.where}, env),
		machinesedit.Command(machinesedit.Deps{Names: service.Names, Resources: machines.ParseResources, Here: m.where}, env),
		machinesprobe.Command(machinesprobe.Deps{Names: service.Names, Overview: overview}, env),
		machinescmd.SSHCommand(machinescmd.SSHDeps{Names: service.Names, Here: m.where}, env),
		machinesadd.Command(machinesadd.Deps{Here: m.where}, env),
		machinesforget.Command(machinesforget.Deps{Names: service.Names, Here: m.where}, env),
		machinesunlock.Command(machinesunlock.Deps{Names: service.Names, Here: m.where}, env),
		machinessupervise.Command(env),
		machinespipe.Command(machinespipe.Deps{Machines: vmPipe{service}}, env),
	)
}

func (m machine) pausing(env kit.Env) func(mac string) []string {
	return func(mac string) []string {
		vms, _ := machineCalls{Service: m.machines(env.Interactive), m: m}.VMsOf(context.Background(), mac)
		var names []string
		for _, vm := range model.Ordered(vms) {
			if vm.State == machines.Running {
				names = append(names, vm.Name)
			}
		}
		return names
	}
}

func createCheck(s *machines.Service) func(ctx context.Context, r machines.CreateRequest) (bool, error) {
	return func(ctx context.Context, r machines.CreateRequest) (bool, error) {
		_, err := s.CheckCreate(ctx, r)
		var unbuilt *machines.NoImageError
		if errors.As(err, &unbuilt) {
			return true, err
		}
		return false, err
	}
}

type vmPipe struct{ *machines.Service }

func (p vmPipe) Check(ctx context.Context, vm string) error {
	_, err := p.CheckPipe(ctx, vm)
	return err
}

// invariant: the machines calls' Deps over today's service: each check the
// leaves ran before, worded by the service's own errors.
type machineCalls struct {
	*machines.Service
	m machine
}

// invariant: a Mac is reached as every later call reaches it, through this
// run's Router over its ssh block; its version is judged against this one's
// by the fleet's own rule.
func (c machineCalls) Reach(ctx context.Context, name string) (model.Added, error) {
	hello, err := c.m.far.router.Hello(ctx, name)
	if err != nil {
		return model.Added{}, err
	}
	update, _ := model.Behind(model.HostView{Name: name, StationVersion: hello.Version}, c.m.stationVersion())
	return model.Added{Name: name, Label: c.m.label(name), Version: hello.Version, Update: update}, nil
}

func (c machineCalls) CheckVM(name, verb string) error {
	_, err := c.VM(name, verb)
	return err
}

func (c machineCalls) CheckRestart(name string) (bool, error) {
	r, err := c.Remote(name)
	if err != nil {
		return false, err
	}
	if r.Kind == catalog.Host {
		return false, nil
	}
	return true, nil
}

func (c machineCalls) Running(ctx context.Context, name string) (bool, error) {
	allMachines, err := c.Fleet(ctx, true)
	if err != nil {
		return false, err
	}
	for _, h := range allMachines.Hosts {
		for _, vm := range h.VMs {
			if vm.Name == name {
				return vm.State == machines.Running, nil
			}
		}
	}
	return false, &machines.UnknownMachineError{Name: name}
}

func (c machineCalls) Restart(ctx context.Context, name string) (machines.RestartReport, error) {
	return c.Service.Restart(ctx, name, nil)
}

func (c machineCalls) CheckRemoteHost(name, verb string) (string, error) {
	m, err := c.RemoteHost(name, verb)
	return m.Name, err
}

func (c machineCalls) Unlock(ctx context.Context, name string, interactive bool) error {
	if !interactive {
		return &machines.NeedsTerminalError{Verb: "machines unlock"}
	}
	return c.Service.Unlock(ctx, name)
}

func (c machineCalls) VMsOf(ctx context.Context, name string) ([]machines.VM, error) {
	out, err := call.Read(ctx, c.m.far.router, coremachine.Ref{Name: name}, machinescalls.List, machinescalls.ListIn{Here: true}, nil)
	if err != nil || len(out.Fleet.Hosts) == 0 {
		return nil, err
	}
	return out.Fleet.Hosts[0].VMs, nil
}

func (c machineCalls) StartOn(ctx context.Context, mac, vm string) error {
	_, err := call.Invoke(ctx, c.m.far.router, coremachine.Ref{Name: mac}, machinescalls.Start, machinescalls.VMIn{VM: vm}, nil)
	return err
}

func (c machineCalls) CheckRemote(name string) error {
	_, err := c.Remote(name)
	return err
}

type allMachinesImages struct{ service *images.Service }

func (i allMachinesImages) KeyPath() string { return i.service.KeyPath() }

func (i allMachinesImages) Latest(ctx context.Context, image string) (disk, version string, err error) {
	r, err := i.service.Latest(ctx, image)
	var unbuilt *images.NotBuiltError
	if errors.As(err, &unbuilt) {
		return "", "", &machines.NoImageError{Image: image}
	}
	if err != nil {
		return "", "", err
	}
	return images.DiskName(r.Name, r.Version), r.Version, nil
}

func createAsker(env kit.Env) kit.Asker {
	if !env.Interactive {
		return nil
	}
	return env.Prompt
}
