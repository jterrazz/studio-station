package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func secretsCommand(env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:   "secrets",
		Short: "The lend list, and what was lent to which VM",
		Group: "fleet",
		Read:  func(*struct{}) bool { return true },
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
}
