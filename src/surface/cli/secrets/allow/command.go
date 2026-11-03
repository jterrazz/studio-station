package secretsallow

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/page"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Secrets *secrets.Service
	Here    page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "allow <name...>",
		Short:   "Add names to the lend list (session:<id>, operator:claude:oauth-token)",
		Example: "  station secrets allow session:openpanel",
		Args:    cobra.MinimumNArgs(1),
		Bind:    bind,
		Run: func(_ context.Context, in *Input, io kit.IO) error {
			if err := d.Secrets.Allow(in.Names); err != nil {
				return err
			}
			return io.Page(pagesecrets.Change("allow", in.Names, secrets.RecallReport{}, d.Here), nil)
		},
	}, env)
}
