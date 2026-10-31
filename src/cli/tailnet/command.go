package tailnetcmd

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

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:     "tailnet",
		Short:   "This machine's tailnet node, its peers, and whether VMs can join",
		Example: "  station tailnet\n  station tailnet --json",
		Group:   "fleet",
		Read:    func(*struct{}) bool { return true },
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			status, err := d.Tailnet.Status(ctx)
			if err != nil {
				return err
			}
			return io.Answer(presenttailnet.Status(status, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
