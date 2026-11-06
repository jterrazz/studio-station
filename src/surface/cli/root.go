package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func NewRoot(env kit.Env, openTUI func(context.Context) error, overview *cobra.Command, children ...*cobra.Command) *cobra.Command {
	// invariant: bare station is the TUI on a terminal, else overview, which
	// it prints piped or with --all or --json (SPEC §4.2).
	root := &cobra.Command{
		Use:   "station",
		Short: "Set up and run this Mac, its VMs and your other Macs",
	}
	var all, json bool
	root.Flags().BoolVar(&all, "all", false, "print the overview with every section unfolded")
	root.Flags().BoolVar(&json, "json", false, "print the overview as JSON")
	root.RunE = func(c *cobra.Command, _ []string) error {
		if handled, err := kit.Delegates(env, c); handled {
			return err
		}
		if env.Screen && !all && !json {
			return openTUI(c.Context())
		}
		for name, on := range map[string]bool{"all": all, "json": json} {
			if on {
				if err := overview.Flags().Set(name, "true"); err != nil {
					return err
				}
			}
		}
		overview.SetContext(c.Context())
		return overview.RunE(overview, nil)
	}
	root.PersistentPreRunE = kit.Aliased(env, kit.Using(env, kit.RefuseOn(env)))
	var machine string
	target := &machine
	if env.On != nil {
		target = &env.On.Machine
	}
	root.PersistentFlags().StringVar(target, "on", "", "run this command on that `machine`")
	cobra.EnableCommandSorting = false
	root.AddGroup(kit.Groups()...)
	root.SetHelpCommand(kit.Help())
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.CompletionOptions.HiddenDefaultCmd = true
	root.SetFlagErrorFunc(kit.FlagError)
	root.AddCommand(overview)
	root.AddCommand(children...)
	return root
}
