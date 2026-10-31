package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func configCommand(env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:   "config",
		Short: "The config items that set this machine up",
		Group: "here",
		Read:  func(*struct{}) bool { return true },
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
}
