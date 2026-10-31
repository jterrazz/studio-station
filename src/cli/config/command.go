package configcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/config"
	presentconfig "github.com/jterrazz/studio-station/src/present/config"
)

type Deps struct {
	Config  *config.Service
	Context string
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "config",
		Short:   "The config items that set this machine up",
		Example: "  station config\n  station config --json",
		Group:   "here",
		Read:    func(*Input) bool { return true },
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			reports, err := d.Config.List(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentconfig.Items(reports, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
