package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func setupCommand(env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:   "setup",
		Short: "Set this machine up: its name, the ~/.zshrc block, the XDG folders",
		Group: "binary",
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
}
