package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func tailnetCommand(env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:   "tailnet",
		Short: "This machine's tailnet node, and its peers",
		Group: "fleet",
		Read:  func(*struct{}) bool { return true },
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
}
