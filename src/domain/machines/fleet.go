package machines

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const fanOutBudget = 5 * time.Second

func (s *Service) Fleet(ctx context.Context, hereOnly bool) (Fleet, error) {
	cfg, err := s.store.Load()
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
	views := make([]HostView, len(others))
	var wg sync.WaitGroup
	for i, n := range others {
		wg.Add(1)
		go func() {
			defer wg.Done()
			views[i] = s.ask(ctx, n, cfg.Hosts[n], nodes[n])
		}()
	}
	wg.Wait()
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

// invariant: the daemon matters only where VMs can exist: on a host with tart; a
// daemon that does not answer degrades the view, never fails it.
func (s *Service) daemonView(ctx context.Context) (info DaemonInfo, live map[string]DaemonVM) {
	if s.o.Here != catalog.Host || s.daemon == nil {
		return DaemonInfo{}, nil
	}
	if _, ok := s.h.Run.LookPath("tart"); !ok {
		return DaemonInfo{}, nil
	}
	version, err := s.daemon.Ping(ctx)
	if err != nil {
		return DaemonInfo{Known: true}, nil
	}
	live, err = s.daemon.VMs(ctx)
	if err != nil {
		return DaemonInfo{Known: true, Running: true, Version: version}, nil
	}
	return DaemonInfo{Known: true, Running: true, Version: version}, live
}

func (s *Service) ask(ctx context.Context, name string, h settings.Host, node Node) HostView {
	silent := HostView{Name: name, User: h.User, Node: node}
	argv := s.o.Remote(name, []string{"machines", "--here", "--json"})
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: argv[0], Args: argv[1:], Timeout: fanOutBudget})
	if err != nil || out.ExitCode != 0 {
		return silent
	}
	var doc WireFleet
	if json.Unmarshal([]byte(out.Stdout), &doc) != nil || len(doc.Hosts) == 0 {
		return silent
	}
	view := doc.Hosts[0].view(name, h.User)
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
