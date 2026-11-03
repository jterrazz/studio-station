package configcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Config *config.Service
	Here   page.Context
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "config",
		Short:   "The config items that set this machine up",
		Example: "  station config\n  station config --json",
		Group:   "here",
		Read:    func(*Input) bool { return true },
		Pages:   true,
		Entry: kit.Entry("station config install", func() []string {
			placed := d.Config.Placed()
			out := make([]string, 0, len(placed))
			for _, it := range placed {
				out = append(out, it.Name)
			}
			return out
		}),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			reports, err := d.Config.List(ctx)
			if err != nil {
				return err
			}
			return io.Page(pageconfig.Items(reports, d.Here), pageconfig.ItemsContract(reports))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
