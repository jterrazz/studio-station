package secretsrecall

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
)

type Deps struct {
	Secrets *secrets.Service
	Here    page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "recall --from <vm> [<name...>]",
		Short:    "Take the lent copies back out of a VM, every name lent to it by default",
		Example:  "  station secrets recall --from api",
		Args:     cobra.ArbitraryArgs,
		Declare:  func(f *pflag.FlagSet, in *Input) { f.StringVar(&in.From, "from", "", "the VM to recall from") },
		Bind:     bind,
		Complete: func([]string, string) []string { return d.Secrets.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := d.Secrets.Recall(ctx, in.From, in.Names)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Recalled(report, d.Here), nil)
		},
	}, env)
}
