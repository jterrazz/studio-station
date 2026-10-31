package tailnetup

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	presenttailnet "github.com/jterrazz/studio-station/src/present/tailnet"
)

type Deps struct {
	Tailnet      *tailnet.Service
	Authenticate func(ctx context.Context) error
	Context      string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "up",
		Short:   "Connect this machine to the tailnet through the userspace daemon",
		Example: "  station tailnet up",
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			if io.Interactive && d.Authenticate != nil && !d.Tailnet.Configured(ctx) {
				if err := d.Authenticate(ctx); err != nil {
					return err
				}
			}
			result, err := d.Tailnet.Up(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presenttailnet.Up(result, d.Context))
		},
	}, env)
}
