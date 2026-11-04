package enginecmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	enginecalls "github.com/jterrazz/studio-station/src/engine/calls"
	"github.com/jterrazz/studio-station/src/page"
	pageengine "github.com/jterrazz/studio-station/src/pages/engine"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
	Zone *time.Location
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:     "engine",
		Short:   "This Mac's engine: it runs the VMs, Awake and the sharing, and serves the other Macs",
		Example: "  station engine\n  station engine stop\n  station engine start",
		Group:   "fleet",
		Read:    func(*struct{}) bool { return true },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			r, err := call.Invoke(ctx, env.Calls, env.Target(), enginecalls.Read, enginecalls.ReadIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pageengine.Read(r, d.Zone, d.Here), r.JSON())
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
