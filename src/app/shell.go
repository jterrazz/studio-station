package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func shellCommand(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:   "shell <shell>",
		Short: "The script the ~/.zshrc block evaluates",
		Group: "",
		Args:  cobra.ArbitraryArgs,
		Run:   func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
	cmd.Hidden = true
	return cmd
}
