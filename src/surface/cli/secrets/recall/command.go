package secretsrecall

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	LendList func() []string
	Here     page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "recall --from <vm> [<name...>]",
		Short:   "Take the lent copies back out of a VM, every name lent to it by default",
		Example: "  station secrets recall --from api",
		Args:    cobra.ArbitraryArgs,
		Declare: func(f *pflag.FlagSet, in *Input) { f.StringVar(&in.From, "from", "", "the VM to recall from") },
		Bind:    bind,
		Asks:    true,
		Verb:    func(*Input) string { return "recall" },
		Ask: func(in *Input) (bool, error) {
			what := "every secret lent to it"
			if len(in.Names) > 0 {
				what = strings.Join(in.Names, ", ")
			}
			return env.Prompt.Confirm(pageaccounts.RecallConfirm(in.From, what))
		},
		Complete: func([]string, string) []string { return d.LendList() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var report accounts.RecallReport
			err := io.Confirmed(func(answer string) (err error) {
				report, err = call.Invoke(ctx, env.Calls, env.Target(), accountscalls.Recall, accountscalls.RecallIn{From: in.From, Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Recalled(report, d.Here), nil)
		},
	}, env)
}
