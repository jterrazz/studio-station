package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

func NewRoot(env kit.Env, openTUI func(context.Context) error, children ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:   "station",
		Short: "Set up and run this machine and the fleet: tools, config, hosts and VMs",
	}
	root.RunE = func(c *cobra.Command, _ []string) error {
		if env.Screen {
			return openTUI(c.Context())
		}
		return kit.Menu(env, c)
	}
	root.AddGroup(
		&cobra.Group{ID: "here", Title: "Here"},
		&cobra.Group{ID: "fleet", Title: "The fleet"},
		&cobra.Group{ID: "shortcuts", Title: "Shortcuts"},
		&cobra.Group{ID: "binary", Title: "The binary"},
	)
	root.SetHelpCommandGroupID("binary")
	root.CompletionOptions.HiddenDefaultCmd = true
	root.SetFlagErrorFunc(kit.FlagError)
	root.AddCommand(children...)
	return root
}
