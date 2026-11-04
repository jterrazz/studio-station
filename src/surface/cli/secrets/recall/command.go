package secretsrecall

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
		Use:      "recall --from <vm> [<name...>]",
		Short:    "Take the lent copies back out of a VM, every name lent to it by default",
		Example:  "  station secrets recall --from api",
		Args:     cobra.ArbitraryArgs,
		Declare:  func(f *pflag.FlagSet, in *Input) { f.StringVar(&in.From, "from", "", "the VM to recall from") },
		Bind:     bind,
		Complete: func([]string, string) []string { return d.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target, accountscalls.Recall, accountscalls.RecallIn{From: in.From, Names: in.Names}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Recalled(report, d.Here), nil)
		},
	}, env)
}
