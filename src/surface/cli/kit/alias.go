package kit

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const aliasNow = "station.alias.now"

func Alias(old string, cmd *cobra.Command, now string) *cobra.Command {
	// value: cmd as the hidden old word of a command for one release (SPEC
	// §4.3): it runs under the old name, help and completion never list it,
	// and each run names the new command on stderr.
	_, rest, _ := strings.Cut(cmd.Use, " ")
	cmd.Use = strings.TrimSpace(old + " " + rest)
	cmd.Hidden = true
	cmd.GroupID = ""
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[aliasNow] = now
	return cmd
}

// invariant: the notice is the run's own words with every old one replaced,
// its flags then its arguments; --json and --page keep stderr empty for the script.
func notice(env Env, c *cobra.Command, args []string) {
	var words []string
	aliased := false
	for p := c; p != nil && p.HasParent(); p = p.Parent() {
		word := p.Name()
		if now, ok := p.Annotations[aliasNow]; ok {
			word, aliased = now, true
		}
		if word != "" {
			words = append([]string{word}, words...)
		}
	}
	if !aliased {
		return
	}
	for _, machine := range []string{"json", "page"} {
		if f := c.Flags().Lookup(machine); f != nil && f.Changed {
			return
		}
	}
	c.Flags().Visit(func(f *pflag.Flag) {
		if f.Value.Type() == "bool" {
			words = append(words, "--"+f.Name)
			return
		}
		if many, ok := f.Value.(pflag.SliceValue); ok {
			for _, v := range many.GetSlice() {
				words = append(words, "--"+f.Name, v)
			}
			return
		}
		words = append(words, "--"+f.Name, f.Value.String())
	})
	words = append(words, args...)
	env.Out.Now("station " + strings.Join(words, " "))
}

func Aliased(env Env, pre func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		if err := pre(c, args); err != nil {
			return err
		}
		notice(env, c, args)
		return nil
	}
}
