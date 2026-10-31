package tailnetdown

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	presenttailnet "github.com/jterrazz/studio-station/src/present/tailnet"
)

type Deps struct {
	Tailnet *tailnet.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "down",
		Short:   "Disconnect this machine from the tailnet and stop the userspace daemon",
		Example: "  station tailnet down",
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			result, err := d.Tailnet.Down(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presenttailnet.Down(result, d.Context))
		},
	}, env)
}
