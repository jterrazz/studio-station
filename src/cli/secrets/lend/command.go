package secretslend

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
		Use:     "lend --to <vm> [<name...>] [--for <duration>]",
		Short:   "Copy (or refresh) lent secrets into a VM, the whole lend list by default",
		Example: "  station secrets lend --to api\n  station secrets lend --to api session:openpanel --for 4h",
		Args:    cobra.ArbitraryArgs,
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.To, "to", "", "the VM to lend to")
			f.StringVar(&in.For, "for", "", "how long the lend lasts, 1m to 168h (default 12h)")
		},
		Bind:     bind,
		Complete: func([]string, string) []string { return d.Secrets.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := d.Secrets.Lend(ctx, in.To, in.Names, in.Term)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Lent(report, d.Here), nil)
		},
	}, env)
}
