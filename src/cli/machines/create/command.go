package machinescreate

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[struct{}]{
		Use:   "create",
		Short: "Make a VM on a host, from an image",

		Bind: func(*struct{}, []string) error { return nil },
		Run:  func(context.Context, *struct{}, kit.IO) error { return kit.NotBuilt() },
	}, env)
	return cmd
}
