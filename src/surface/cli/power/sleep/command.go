package powersleep

import (
	"context"

	"github.com/spf13/cobra"

	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "sleep",
		Short:   "End the Power session now: the Mac sleeps as macOS decides",
		Example: "  station power sleep",
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			s, err := call.Invoke(ctx, env.Calls, env.Target(), awakecalls.Sleep, awakecalls.SleepIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagepower.Slept(s, d.Here), nil)
		},
	}, env)
}
