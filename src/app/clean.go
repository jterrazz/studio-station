package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func cleanCommand(env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:   "clean",
		Short: "What can be reclaimed, with its size, and the cleaning of it",
		Group: "here",
		Read:  func(*struct{}) bool { return true },
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
}
