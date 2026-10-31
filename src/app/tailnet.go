package app

import (
	"context"
	"slices"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	tailnetcmd "github.com/jterrazz/studio-station/src/cli/tailnet"
	tailnetdown "github.com/jterrazz/studio-station/src/cli/tailnet/down"
	tailnetup "github.com/jterrazz/studio-station/src/cli/tailnet/up"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
)

func (m machine) tailnet(interactive bool) *tailnet.Service {
	return tailnet.New(m.host, m.store, settings.NewKeychain(m.host.Run).On(m.kind), tailnet.Options{
		Paths:       m.paths,
		APIURL:      m.or("STATION_TAILSCALE_API_URL", "https://api.tailscale.com"),
		Interactive: interactive,
	})
}

func tailnetCommand(env kit.Env, m machine) *cobra.Command {
	service := m.tailnet(env.Interactive)
	return tailnetcmd.Command(tailnetcmd.Deps{Tailnet: service, Context: m.context}, env,
		tailnetup.Command(tailnetup.Deps{Tailnet: service, Context: m.context}, env),
		tailnetdown.Command(tailnetdown.Deps{Tailnet: service, Context: m.context}, env),
	)
}

type fleetTailnet struct{ service *tailnet.Service }

func (t fleetTailnet) Mint(ctx context.Context) (string, error) {
	key, err := t.service.Mint(ctx, tailnet.MintRequest{})
	return key.Reveal(), err
}

func (t fleetTailnet) Remove(ctx context.Context, nodeID string) error {
	return t.service.Remove(ctx, nodeID)
}

func (t fleetTailnet) Peers(ctx context.Context) (map[string]machines.Node, error) {
	roster, err := t.service.Roster(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]machines.Node, len(roster))
	for _, p := range roster {
		// invariant: a name two nodes share (a VM made again under a leftover's
		// name) is the online one's.
		if held, ok := nodes[p.Name]; ok && held.Online {
			continue
		}
		nodes[p.Name] = machines.Node{Online: p.Online, IP: p.IP, StationVM: slices.Contains(p.Tags, tailnet.VMTag)}
	}
	return nodes, nil
}

func (t fleetTailnet) Argv(args ...string) []string { return t.service.Argv(args...) }

func (t fleetTailnet) Mode() string { return t.service.Mode() }
