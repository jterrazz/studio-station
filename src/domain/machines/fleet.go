package machines

import (
	"context"
	"slices"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) Fleet(ctx context.Context, hereOnly bool) (Fleet, error) {
	return s.FleetAsItFills(ctx, hereOnly, nil)
}

func (s *Service) FleetAsItFills(ctx context.Context, hereOnly bool, sofar func(Fleet)) (Fleet, error) {
	// invariant: sofar sees this host first, then this host and every other
	// Mac that has answered, in name order, after each answer; the fleet
	// returned holds every Mac, a silent one as unreachable.
	cfg, err := s.prefs()
	if err != nil {
		return Fleet{}, err
	}
	nodes := s.nodeMap(ctx)
	name := s.here(cfg)
	info, live := s.daemonView(ctx)
	vms, err := s.vms(ctx, name, live)
	if err != nil {
		return Fleet{}, err
	}
	for i, vm := range vms {
		if _, own := nodes[vm.Name]; own && vm.Lifetime == AlwaysOn {
			vms[i].Device = vm.Name
		}
	}
	user := cfg.Hosts[cfg.Here].User
	if user == "" {
		user = s.h.Env.User()
	}
	here := HostView{
		Name: name, User: user, StationVersion: s.o.Version, Here: true, Reachable: true, Node: nodes[name], Daemon: info, VMs: vms,
	}
	if hereOnly || s.o.Here != catalog.Host {
		return Fleet{Hosts: []HostView{here}}, nil
	}
	others := s.OtherHosts()
	answers := map[string]HostView{}
	fleet := func(all bool) Fleet {
		f := Fleet{Hosts: []HostView{here}, Read: s.h.Clock.Now()}
		for _, n := range others {
			if _, ok := answers[n]; ok || all {
				f.Hosts = append(f.Hosts, s.view(n, cfg.Hosts[n], nodes[n], answers))
			}
		}
		return f
	}
	if sofar != nil {
		sofar(fleet(false))
	}
	if s.o.Ask != nil && len(others) > 0 {
		s.o.Ask(ctx, others, func(mac string, view HostView) {
			answers[mac] = view
			if sofar != nil {
				sofar(fleet(false))
			}
		})
	}
	return fleet(true), nil
}

func (s *Service) nodeMap(ctx context.Context) map[string]Node {
	if s.nodes == nil || s.o.Here != catalog.Host {
		return nil
	}
	nodes, err := s.nodes.Peers(ctx)
	if err != nil {
		return nil
	}
	return nodes
}

// invariant: the daemon matters only where VMs can exist: on a host with its hypervisor; a
// daemon that does not answer degrades the view, never fails it.
func (s *Service) daemonView(ctx context.Context) (info DaemonInfo, live map[string]DaemonVM) {
	if s.o.Here != catalog.Host || s.daemon == nil {
		return DaemonInfo{}, nil
	}
	if !s.engine.Installed() {
		return DaemonInfo{}, nil
	}
	version, err := s.daemon.Ping(ctx)
	if err != nil {
		_, missing := s.h.FS.Stat(catalog.EnginePlist(s.h.Env.Home()))
		return DaemonInfo{Known: true, Installed: missing == nil}, nil
	}
	live, err = s.daemon.VMs(ctx)
	if err != nil {
		return DaemonInfo{Known: true, Running: true, Version: version}, nil
	}
	return DaemonInfo{Known: true, Running: true, Version: version}, live
}

func (s *Service) view(name string, h model.Host, node Node, answers map[string]HostView) HostView {
	far, ok := answers[name]
	if !ok {
		silent := HostView{Name: name, User: h.User, Node: node, Answered: h.Answered}
		for _, vm := range h.VMs {
			silent.VMs = append(silent.VMs, VM{
				Name: vm.Name, Host: name, Lifetime: vm.Lifetime, State: model.UnknownState,
				Resources: Resources{CPUs: vm.CPUs, MemoryBytes: vm.MemoryBytes},
			})
		}
		return silent
	}
	view := HostView{Name: name, User: h.User, StationVersion: far.StationVersion, Reachable: true, Node: far.Node, Daemon: far.Daemon}
	for _, vm := range far.VMs {
		vm.Host = name
		view.VMs = append(view.VMs, vm)
	}
	if node.IP != "" || node.Online {
		view.Node = node
	}
	return view
}

func (s *Service) Running(ctx context.Context) (map[string]catalog.MachineKind, error) {
	_, live := s.daemonView(ctx)
	vms, err := s.vms(ctx, "", live)
	if err != nil {
		return nil, err
	}
	running := map[string]catalog.MachineKind{}
	for _, vm := range vms {
		if vm.State == Running {
			running[vm.Name] = vm.Kind
		}
	}
	return running, nil
}

func (s *Service) Remembers(mac, vm string) bool {
	cfg, err := s.prefs()
	if err != nil {
		return false
	}
	return slices.ContainsFunc(cfg.Hosts[mac].VMs, func(v model.SeenVM) bool { return v.Name == vm })
}
