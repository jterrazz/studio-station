package kit

import (
	"strings"

	"github.com/spf13/cobra"
)

func Alias(old, became string, now *cobra.Command, env Env) *cobra.Command {
	// invariant: an alias is an old word kept for one release: hidden in help
	// and completion, it runs now, the command the word became, with the same
	// arguments, and says so on stderr, except to a reader of --json.
	now.Use = old + strings.TrimPrefix(now.Use, strings.Fields(now.Use)[0])
	now.Hidden, now.GroupID, now.Example = true, "", ""
	run := now.PreRunE
	now.PreRunE = func(c *cobra.Command, args []string) error {
		if json, _ := c.Flags().GetBool("json"); !json && env.Out != nil {
			env.Out.Now(strings.Join(append([]string{"station", became}, args...), " "))
		}
		if run != nil {
			return run(c, args)
		}
		return nil
	}
	return now
}
