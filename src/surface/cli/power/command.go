package powercmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/awake"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "power",
		Short:   "Keep this host awake for a while: the session, its guards and the reading",
		Example: "  station power\n  station power awake --lid --for 2h\n  station power sleep",
		Group:   kit.OnAMachine,
		Read:    func(*Input) bool { return true },
		Live:    true,
		Pages:   true,
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			updates, err := call.Follow(ctx, env.Calls, env.Target(), awakecalls.Read, awakecalls.ReadIn{}, nil)
			if err != nil {
				return err
			}
			if io.CanLive() {
				return io.LivePage(kit.Updates(updates, func(r awake.Report) page.Page { return pagepower.Page(r, d.Here) }))
			}
			var last awake.Report
			for u := range updates {
				last = u.Value
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return io.PageRecords(pagepower.Page(last, d.Here), pagepower.Records(last), pagepower.Contract(last))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
