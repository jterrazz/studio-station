package accountscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "accounts",
		Short:   "Who has access to what, here and in this host's VMs: identities and scopes, never a token",
		Example: "  station accounts\n  station accounts --json",
		Group:   "fleet",
		Read:    func(*Input) bool { return true },
		Live:    true,
		Pages:   true,
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			updates, err := call.Follow(ctx, env.Calls, env.Target, accountscalls.Read, accountscalls.ReadIn{}, nil)
			if err != nil {
				return err
			}
			if io.CanLive() {
				return io.LivePage(kit.Updates(updates, func(r accounts.Report) page.Page {
					return pageaccounts.Page(r, d.Here)
				}))
			}
			var last accounts.Report
			for u := range updates {
				last = u.Value
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return io.PageRecords(pageaccounts.Page(last, d.Here), pageaccounts.Records(last), pageaccounts.Contract(last))
		},
	}, env)
}
