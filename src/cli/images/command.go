package imagescmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/images"
	presentimages "github.com/jterrazz/studio-station/src/present/images"
)

type Deps struct {
	Images  *images.Service
	Context string
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:     "images",
		Short:   "The VM images on this host, with their versions and the VMs made from each",
		Example: "  station images\n  station images --json",
		Group:   "fleet",
		Read:    func(*struct{}) bool { return true },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			reports, err := d.Images.List(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentimages.List(reports, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
