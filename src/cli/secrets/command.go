package secretscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	presentsecrets "github.com/jterrazz/studio-station/src/present/secrets"
)

type Deps struct {
	Secrets *secrets.Service
	Context string
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "secrets",
		Short:   "The lend list, and what was lent to which VM",
		Example: "  station secrets\n  station secrets --json",
		Group:   "fleet",
		Read:    func(*Input) bool { return true },
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			report, err := d.Secrets.List(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presentsecrets.List(report, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
