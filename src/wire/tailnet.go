package wire

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/adapter/tailscale"
	"github.com/jterrazz/studio-station/src/ports/contract"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	tailnetcmd "github.com/jterrazz/studio-station/src/surface/cli/tailnet"
)

func (m machine) exposure(ctx context.Context) services.Exposure {
	ready, err := m.tailnet().Ready(ctx)
	if err != nil {
		return services.ExposureOf(false, "")
	}
	return services.ExposureOf(ready.On, ready.Host)
}

func (m machine) tailnet() contract.Tailnet {
	if m.world != nil {
		return m.world.tailnet()
	}
	return m.tailscale()
}

func (m machine) tailscale() *tailscale.CLI {
	return tailscale.New(m.host.Run, m.host.FS, m.systemRoot())
}

// value: the tailnet as a page reads it, this node and what it shares in
// one pass, each share told station's when the services policy owns it.
type tailnetStatus struct{ m machine }

func (m machine) tailnetStatus() tailnetStatus { return tailnetStatus{m: m} }

func (t tailnetStatus) Status(ctx context.Context) (ports.Status, error) {
	g, _ := t.m.stores.services().Get()
	p := services.PolicyOf(g)
	owns := func(s ports.Share) bool {
		return p.InRange(s.Port) || (p.HTTPS == services.HTTPSPath && s.Port == 443 && s.Path != "/")
	}
	tn := t.m.tailnet()
	self, err := tn.Self(ctx)
	var shared []ports.Share
	if err == nil {
		shared, _ = tn.Shared(ctx)
	}
	return ports.StatusOf(self, err, shared, owns), nil
}

func tailnetCommand(env kit.Env, m machine) *cobra.Command {
	return tailnetcmd.Command(tailnetcmd.Deps{Here: m.where}, env)
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
