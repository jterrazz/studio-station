package toolscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/apps"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the catalogue tools placed on this machine, for completion.
	Names func() []string
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
		Entry:   kit.Entry("station tools install", d.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			if io.CanLive() {
				updates, err := call.Follow(ctx, env.Calls, env.Target, appscalls.List, appscalls.ListIn{}, nil)
				if err != nil {
					return err
				}
				if err := io.LivePage(kit.Updates(updates, func(r []apps.Report) page.Page {
					return pagetools.Catalogue(r, d.Here)
				})); err != nil {
					return err
				}
				return ctx.Err()
			}
			reports, err := call.Read(ctx, env.Calls, env.Target, appscalls.List, appscalls.ListIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagetools.Catalogue(reports, d.Here), pagetools.CatalogueContract(reports))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
