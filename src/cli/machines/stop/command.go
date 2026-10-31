package machinesstop

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:   "stop <vm>",
		Short: "Stop a lasting VM; it stays stopped",
		Args:  cobra.ExactArgs(1),
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
	return cmd
}
