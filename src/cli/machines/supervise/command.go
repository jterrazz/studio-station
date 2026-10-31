package machinessupervise

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:   "supervise <vm>",
		Short: "Run a VM for its launchd agent",
		Args:  cobra.ExactArgs(1),
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
	cmd.Hidden = true
	return cmd
}
