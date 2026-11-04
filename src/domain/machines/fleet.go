package machines

import (
	"context"
	"sort"

	model "github.com/jterrazz/studio-station/src/machines"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) Fleet(ctx context.Context, hereOnly bool) (Fleet, error) {
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
	user := cfg.Hosts[cfg.Here].User
	if user == "" {
		user = s.h.Env.User()
	}
	fleet := Fleet{Hosts: []HostView{{
		Name: name, User: user, StationVersion: s.o.Version, Here: true, Reachable: true, Node: nodes[name], Daemon: info, VMs: vms,
	}}}
	if hereOnly || s.o.Here != catalog.Host {
		return fleet, nil
	}
	others := make([]string, 0, len(cfg.Hosts))
	for n := range cfg.Hosts {
		if n != cfg.Here {
			others = append(others, n)
		}
	}
	sort.Strings(others)
	var answers map[string]HostView
	if s.o.Ask != nil {
		answers = s.o.Ask(ctx, others)
	}
	views := make([]HostView, len(others))
	for i, n := range others {
		views[i] = s.view(n, cfg.Hosts[n], nodes[n], answers)
	}
	fleet.Hosts = append(fleet.Hosts, views...)
	return fleet, nil
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
		_, missing := s.h.FS.Stat(catalog.DaemonPlist(s.h.Env.Home()))
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
		return HostView{Name: name, User: h.User, Node: node}
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
