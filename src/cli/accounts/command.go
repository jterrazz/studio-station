package accountscmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"
)

type Deps struct {
	Accounts *accounts.Service
	Here     page.Context
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
			if io.CanLive() {
				return io.LivePage(kit.Pages(d.Accounts.Read(ctx), func(r accounts.Report) page.Page {
					return pageaccounts.Page(r, d.Here)
				}))
			}
			var last accounts.Report
			for r := range d.Accounts.Read(ctx) {
				last = r
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return io.PageRecords(pageaccounts.Page(last, d.Here), pageaccounts.Records(last), pageaccounts.Contract(last))
		},
	}, env)
}
