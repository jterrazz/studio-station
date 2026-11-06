package wire

import (
	"context"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/adapter/cloudflare"
	"github.com/jterrazz/studio-station/src/ports/adapter/funnel"
	"github.com/jterrazz/studio-station/src/ports/adapter/lan"
	"github.com/jterrazz/studio-station/src/ports/adapter/tailscale"
	"github.com/jterrazz/studio-station/src/ports/contract"
)

func (m machine) exposure(ctx context.Context) ports.Exposure {
	ready, err := m.tailnet().Ready(ctx)
	if err != nil {
		return ports.ExposureOf(false, "")
	}
	return ports.ExposureOf(ready.On, ready.Host)
}

func (m machine) tailnet() contract.Tailnet {
	if m.world != nil {
		return m.world.tailnet()
	}
	return m.tailnetCLI()
}

func (m machine) routes() contract.PublicRoutes {
	var config funnel.Config = m.tailnetCLI()
	if m.world != nil {
		config = m.world.funnel()
	}
	return contract.PublicRoutes{cloudflare.Route{}, funnel.New(config), lan.Route{}}
}

func (m machine) tailnetCLI() *tailscale.CLI {
	return tailscale.New(m.host.Run, m.host.FS, m.systemRoot())
}

// value: the tailnet as a page reads it, this node and what it shares in
// one pass, each share told station's when the ports policy owns it.
type tailnetStatus struct{ m machine }

func (m machine) tailnetStatus() tailnetStatus { return tailnetStatus{m: m} }

func (t tailnetStatus) Status(ctx context.Context) (ports.Status, error) {
	g, _ := t.m.stores.services().Get()
	p := ports.PolicyOf(g)
	owns := func(s ports.Share) bool {
		return p.InRange(s.Port) || (p.HTTPS == ports.HTTPSPath && s.Port == 443 && s.Path != "/")
	}
	tn := t.m.tailnet()
	self, err := tn.Self(ctx)
	var shared []ports.Share
	if err == nil {
		shared, _ = tn.Shared(ctx)
	}
	return ports.StatusOf(self, err, shared, owns), nil
}

type hostNodes struct{ tailnet contract.Tailnet }

func (t hostNodes) Peers(ctx context.Context) (map[string]machines.Node, error) {
	self, err := t.tailnet.Self(ctx)
	if err != nil {
		return nil, err
	}
	roster := self.Peers
	if self.Name != "" {
		roster = append([]ports.Node{self.Node}, roster...)
	}
	nodes := make(map[string]machines.Node, len(roster))
	for _, p := range roster {
		if held, ok := nodes[p.Name]; ok && held.Online {
			continue
		}
		nodes[p.Name] = machines.Node{Online: p.Online, IP: p.IP}
	}
	return nodes, nil
}
