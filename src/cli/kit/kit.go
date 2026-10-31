package kit

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/prompt"
)

type Env struct {
	Out         *output.Writer
	Prompt      prompt.Prompter
	Interactive bool
	Screen      bool
	On          *On
}

type IO struct {
	JSON          bool
	Yes           bool
	Interactive   bool
	Prompt        prompt.Prompter
	ZshCompletion func(w io.Writer) error
	out           *output.Writer
}

func (o IO) Answer(a present.Answer) error { return o.out.Answer(a, o.JSON) }

type Leaf[I any] struct {
	Use, Short, Example string
	Group               string
	Args                cobra.PositionalArgs
	ValidArgs           []string
	RawArgs             bool
	Declare             func(f *pflag.FlagSet, in *I)
	Bind                func(in *I, args []string) error
	Validate            func(ctx context.Context, in *I) error
	Read                func(in *I) bool
	Destructive         func(in *I) bool
	Confirm             func(in *I) string
	Complete            func(args []string, toComplete string) []string
	Run                 func(ctx context.Context, in *I, io IO) error
}

func Command[I any](l Leaf[I], env Env) *cobra.Command {
	in := new(I)
	var wantJSON, yes bool
	args := l.Args
	if args == nil {
		args = cobra.NoArgs
	}
	cmd := &cobra.Command{
		Use:                l.Use,
		Short:              l.Short,
		Example:            l.Example,
		GroupID:            l.Group,
		ValidArgs:          l.ValidArgs,
		DisableFlagParsing: l.RawArgs,
	}
	cmd.Args = func(c *cobra.Command, a []string) error {
		if err := args(c, a); err != nil {
			return fail.Usage(err.Error(), helpHint(c))
		}
		return nil
	}
	if l.Complete != nil {
		cmd.ValidArgsFunction = func(_ *cobra.Command, a []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return l.Complete(a, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
	}
	if l.Declare != nil {
		l.Declare(cmd.Flags(), in)
	}
	if l.Read != nil {
		cmd.Flags().BoolVar(&wantJSON, "json", false, "print the answer as JSON")
	}
	if l.Destructive != nil {
		cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do it without asking")
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		if l.RawArgs && len(a) > 0 && (a[0] == "-h" || a[0] == "--help") {
			return c.Help()
		}
		if machine, ok := env.On.Target(); ok {
			return Translate(env.On.Delegate(c.Context(), machine, env.Interactive), path(c))
		}
		err := run(c.Context(), c, l, in, a, env, IO{
			JSON:          wantJSON,
			Yes:           yes,
			Interactive:   env.Interactive,
			Prompt:        env.Prompt,
			ZshCompletion: c.Root().GenZshCompletion,
			out:           env.Out,
		})
		if err != nil {
			return Translate(err, path(c))
		}
		return nil
	}
	return cmd
}

func run[I any](ctx context.Context, c *cobra.Command, l Leaf[I], in *I, args []string, env Env, o IO) error {
	if l.Bind != nil {
		if err := l.Bind(in, args); err != nil {
			return err
		}
	}
	if l.Validate != nil {
		if err := l.Validate(ctx, in); err != nil {
			return err
		}
	}
	if l.Destructive != nil && l.Destructive(in) {
		question := c.Name() + "?"
		if l.Confirm != nil {
			question = l.Confirm(in)
		}
		if err := gate(c.Name(), question, o); err != nil {
			return err
		}
	}
	if l.Read != nil && l.Read(in) && !o.JSON {
		stop := env.Out.Wait("reading")
		defer stop()
	}
	return l.Run(ctx, in, o)
}

func gate(verb, question string, o IO) error {
	switch {
	case o.Yes:
		return nil
	case o.Interactive:
		ok, err := o.Prompt.Confirm(question)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return fail.Interrupted()
			}
			return fail.Failed(err.Error(), "re-run with --yes to skip the question", err)
		}
		if !ok {
			return fail.Failed(verb+" cancelled", "re-run with --yes to skip the question", nil)
		}
		return nil
	default:
		return fail.Usage("refusing to "+verb+" without --yes", "re-run with --yes")
	}
}

func Group(use, short, groupID string, env Env, children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		GroupID: groupID,
		Args:    cobra.ArbitraryArgs,
	}
	cmd.AddCommand(children...)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return Menu(env, c)
		}
		hint := helpHint(c)
		if s := c.SuggestionsFor(args[0]); len(s) > 0 {
			hint += didYouMean(s)
		}
		return fail.Usage("unknown command \""+args[0]+"\" for \""+c.CommandPath()+"\"", hint)
	}
	return cmd
}

func Menu(env Env, cmd *cobra.Command) error {
	env.Out.Menu(func(w io.Writer) {
		cmd.SetOut(w)
		defer cmd.SetOut(nil)
		_ = cmd.Help()
	})
	return fail.Silent(fail.CodeUsage)
}

func Refuse(message, hint string) error {
	return fail.Usage(message, hint)
}

func NotBuilt() error {
	return fail.Failed("not built yet", "run station --help", nil)
}

func FlagError(c *cobra.Command, err error) error {
	return fail.Usage(err.Error(), helpHint(c))
}

func helpHint(c *cobra.Command) string {
	return "run " + c.CommandPath() + " --help"
}

func path(c *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(c.CommandPath(), c.Root().Name()))
}

func didYouMean(suggestions []string) string {
	return " (did you mean \"" + strings.Join(suggestions, "\" or \"") + "\"?)"
}
