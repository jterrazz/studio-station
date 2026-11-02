package toolscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
	presenttools "github.com/jterrazz/studio-station/src/present/tools"
)

type Deps struct {
	Tools   *tools.Service
	Context string
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "tools",
		Short:   "The catalogue tools this machine may hold, agent skills included",
		Example: "  station tools\n  station tools --json",
		Group:   "here",
		Read:    func(*Input) bool { return true },
		Live:    true,
		Entry: kit.Entry("station tools install", func() []string {
			placed := d.Tools.Placed()
			out := make([]string, 0, len(placed))
			for _, t := range placed {
				out = append(out, t.Name)
			}
			return out
		}),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				if err := io.Live(kit.Answers(d.Tools.Watch(ctx), func(r []tools.Report) present.Answer {
					return presenttools.Catalogue(r, d.Context)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			reports, err := d.Tools.Inventory(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presenttools.Catalogue(reports, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
