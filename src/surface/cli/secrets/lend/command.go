package secretslend

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	LendList func() []string
	Here     page.Context
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
		Complete: func([]string, string) []string { return d.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target(), accountscalls.Lend, accountscalls.LendIn{To: in.To, Names: in.Names, Term: in.Term}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Lent(report, d.Here), nil)
		},
	}, env)
}
