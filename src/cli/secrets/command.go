package secretscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
)

type Deps struct {
	Secrets *secrets.Service
	Here    page.Context
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "secrets",
		Short:   "The lend list, and what was lent to which VM",
		Example: "  station secrets\n  station secrets --json",
		Group:   "fleet",
		Read:    func(*Input) bool { return true },
		Pages:   true,
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			report, err := d.Secrets.List(ctx)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.List(report, d.Here), pagesecrets.ListContract(report))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
