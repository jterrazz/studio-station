package tailnetcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagetailnet "github.com/jterrazz/studio-station/src/pages/tailnet"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "tailnet",
		Short:   "This host's node in the Tailscale app, its peers and what it serves",
		Example: "  station tailnet\n  station tailnet --json",
		Group:   kit.OnAMachine,
		Read:    func(*struct{}) bool { return true },
		Pages:   true,
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			status, err := call.Invoke(ctx, env.Calls, env.Target(), portscalls.Tailnet, portscalls.TailnetIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagetailnet.Status(status, d.Here), pagetailnet.StatusContract(status))
		},
	}, env)
}
