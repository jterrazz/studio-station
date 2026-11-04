package imagescmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	"github.com/jterrazz/studio-station/src/page"
	pageimages "github.com/jterrazz/studio-station/src/pages/images"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:     "images",
		Short:   "The VM images on this host, with their versions and the VMs made from each",
		Example: "  station images\n  station images --json",
		Group:   kit.ImagesAndEngine,
		Read:    func(*struct{}) bool { return true },
		Pages:   true,
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			reports, err := call.Invoke(ctx, env.Calls, env.Target(), imagescalls.List, imagescalls.ListIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pageimages.List(reports, d.Here), pageimages.ListContract(reports))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
