package toolscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
)

type Deps struct {
	Tools *tools.Service
	Here  page.Context
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "tools",
		Short:   "The catalogue tools this machine may hold, agent skills included",
		Example: "  station tools\n  station tools --json",
		Group:   "here",
		Read:    func(*Input) bool { return true },
		Live:    true,
		Pages:   true,
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
				if err := io.LivePage(kit.Pages(d.Tools.Watch(ctx), func(r []tools.Report) page.Page {
					return pagetools.Catalogue(r, d.Here)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			reports, err := d.Tools.Inventory(ctx)
			if err != nil {
				return err
			}
			return io.Page(pagetools.Catalogue(reports, d.Here), pagetools.CatalogueContract(reports))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
