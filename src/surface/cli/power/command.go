package powercmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Power *power.Service
	Here  page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:      "power",
		Short:    "Keep this host awake for a while: the session, its guards and the reading",
		Example:  "  station power\n  station power awake --lid --for 2h\n  station power sleep",
		Group:    "here",
		Read:     func(*Input) bool { return true },
		Live:     true,
		Pages:    true,
		Validate: func(context.Context, *Input) error { return d.Power.Check() },
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				return io.LivePage(kit.Pages(d.Power.Read(ctx), func(r power.Report) page.Page { return pagepower.Page(r, d.Here) }))
			}
			var last power.Report
			for r := range d.Power.Read(ctx) {
				last = r
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
