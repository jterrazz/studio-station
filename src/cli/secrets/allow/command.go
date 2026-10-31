package secretsallow

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	presentsecrets "github.com/jterrazz/studio-station/src/present/secrets"
)

type Deps struct {
	Secrets *secrets.Service
	Context string
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
			return io.Answer(presentsecrets.Change("allow", in.Names, secrets.RecallReport{}, d.Context))
		},
	}, env)
}
