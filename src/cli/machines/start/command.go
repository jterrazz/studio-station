package machinesstart

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:   "start <vm>",
		Short: "Start a stopped lasting VM",
		Args:  cobra.ExactArgs(1),
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
	return cmd
}
