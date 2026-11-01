package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	tailnetcmd "github.com/jterrazz/studio-station/src/cli/tailnet"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
)

func (m machine) exposure(ctx context.Context) services.Exposure {
	self, err := m.tailnet().Self(ctx)
	if err != nil {
		return services.ExposureOf(false, "")
	}
	return services.ExposureOf(self.ServeOn, self.DNSName)
}

func (m machine) tailnet() *tailnet.Service {
	cfg, _ := m.store.Load()
	p := services.PolicyOf(cfg.Services)
	return tailnet.New(m.host, func(e tailnet.ServeEntry) bool {
		return p.InRange(e.Port) || (p.HTTPS == services.HTTPSPath && e.Port == 443 && e.Path != "/")
	})
}

func tailnetCommand(env kit.Env, m machine) *cobra.Command {
	return tailnetcmd.Command(tailnetcmd.Deps{Tailnet: m.tailnet(), Context: m.context}, env)
}

type hostNodes struct{ service *tailnet.Service }

func (t hostNodes) Peers(ctx context.Context) (map[string]machines.Node, error) {
	roster, err := t.service.Roster(ctx)
	if err != nil {
		return nil, err
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
