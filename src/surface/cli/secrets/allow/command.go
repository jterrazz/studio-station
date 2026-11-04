package secretsallow

import (
	"context"

	"github.com/spf13/cobra"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "allow <name...>",
		Short:   "Add names to the lend list (session:<id>, operator:claude:oauth-token)",
		Example: "  station secrets allow session:openpanel",
		Args:    cobra.MinimumNArgs(1),
		Bind:    bind,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target(), accountscalls.Allow, accountscalls.NamesIn{Names: in.Names}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagesecrets.Change("allow", in.Names, report, d.Here), nil)
		},
	}, env)
}
