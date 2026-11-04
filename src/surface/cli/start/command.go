package startcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	"github.com/jterrazz/studio-station/src/page"
	pageoutcome "github.com/jterrazz/studio-station/src/pages/outcome"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "start",
		Short:   "Start station on this host again, and the lasting VMs whose wish is running",
		Example: "  station start",
		Group:   "fleet",
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target, enginecalls.Start, enginecalls.StartIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pageoutcome.Start(report, d.Here), nil)
		},
	}, env)
}
