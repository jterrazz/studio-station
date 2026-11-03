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
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/present"
)

type Asker interface {
	Confirm(c page.Confirm) (bool, error)
	Form(f page.Form) (map[string]string, error)
}

type Env struct {
	Out         *output.Writer
	Prompt      Asker
	Interactive bool
	Screen      bool
	On          *On
	Stdin       io.Reader
	Stdout      io.Writer
}

type IO struct {
	JSON          bool
	PageJSON      bool
	Yes           bool
	Interactive   bool
	Prompt        Asker
	ZshCompletion func(w io.Writer) error
	out           *output.Writer
	in            io.Reader
	raw           io.Writer
}

func (o IO) Streams() (io.Reader, io.Writer) { return o.in, o.raw }

func (o IO) Answer(a present.Answer) error { return o.out.Answer(a, o.JSON) }

func (o IO) CanLive() bool {
	// invariant: a read goes live only on a terminal, without --json and without
	// NO_COLOR; everywhere else it prints one complete document.
	return !o.JSON && !o.PageJSON && o.out.CanLive()
}

func (o IO) Live(answers <-chan present.Answer) error { return o.out.Live(answers) }

func (o IO) Page(p page.Page, contract any) error {
	return o.out.Page(p, contract, output.Want{JSON: o.JSON, Page: o.PageJSON})
}

// PagePiped prints a page whose piped form is a fixed text, for a script to read whole.
func (o IO) PagePiped(p page.Page, contract any, piped string) error {
	return o.out.Page(p, contract, output.Want{JSON: o.JSON, Page: o.PageJSON, Piped: []byte(piped)})
}

func (o IO) LivePage(pages <-chan page.Page) error { return o.out.LivePage(pages) }

func (o IO) Raw(p []byte) error              { return o.out.Raw(p) }
func (o IO) Note(text string)                { o.out.Note(text) }
func (o IO) Wait(label string) (stop func()) { return o.out.Wait(label) }

type Leaf[I any] struct {
	Use, Short, Example string
	Group               string
	Plumbing            bool
	Args                cobra.PositionalArgs
	ValidArgs           []string
	RawArgs             bool
	Declare             func(f *pflag.FlagSet, in *I)
	Bind                func(in *I, args []string) error
	Validate            func(ctx context.Context, in *I) error
	Read                func(in *I) bool
	Destructive         func(in *I) bool
	Confirm             func(in *I) string
	Verb                func(in *I) string
	Ask                 func(in *I) (bool, error)
	Complete            func(args []string, toComplete string) []string
	Entry               func(word string) string
	Run                 func(ctx context.Context, in *I, io IO) error
	// value: a leaf that draws its reading as it fills needs no spinner.
	Live bool
	// value: a leaf whose answer is a page also prints it as station.page/v1.
	Pages bool
}

func Command[I any](l Leaf[I], env Env) *cobra.Command {
	in := new(I)
	var wantJSON, wantPage, yes bool
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
		Hidden:             l.Plumbing,
		Annotations:        map[string]string{owned: "leaf"},
	}
	cmd.Args = func(c *cobra.Command, a []string) error {
		if err := args(c, a); err != nil {
			hint := helpHint(c)
			if l.Entry != nil && len(a) > 0 {
				if h := l.Entry(a[0]); h != "" {
					hint = h
				}
			}
			return fail.Usage(err.Error(), hint)
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
	if l.Pages {
		cmd.Flags().BoolVar(&wantPage, "page", false, "print the page as station.page/v1")
		_ = cmd.Flags().MarkHidden("page")
	}
	if l.Destructive != nil {
		cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do it without asking")
	}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		if l.RawArgs && len(a) > 0 && (a[0] == "-h" || a[0] == "--help") {
			return c.Help()
		}
		if machine, ok := env.On.Target(); ok {
			if err := env.On.Delegate(c.Context(), machine, env.Interactive); err != nil {
				return Translate(err, path(c))
			}
			env.On.refresh(c.Context(), path(c))
			return nil
		}
		o := IO{
			JSON:          wantJSON,
			PageJSON:      wantPage,
			Yes:           yes,
			Interactive:   env.Interactive,
			Prompt:        env.Prompt,
			ZshCompletion: c.Root().GenZshCompletion,
			out:           env.Out,
		}
		if l.Plumbing {
			o.in, o.raw = env.Stdin, env.Stdout
		}
		err := run(c.Context(), c, l, in, a, env, o)
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
		ask := func() (bool, error) { return o.Prompt.Confirm(page.Confirm{Question: question}) }
		if l.Ask != nil {
			ask = func() (bool, error) { return l.Ask(in) }
		}
		verb := c.Name()
		if l.Verb != nil {
			verb = l.Verb(in)
		}
		if err := gate(verb, ask, o); err != nil {
			return err
		}
	}
	if l.Read != nil && l.Read(in) && !o.JSON && !o.PageJSON && (!l.Live || !o.CanLive()) {
		stop := env.Out.Wait("reading")
		defer stop()
	}
	return l.Run(ctx, in, o)
}

func gate(verb string, ask func() (bool, error), o IO) error {
	switch {
	case o.Yes:
		return nil
	case o.Interactive:
		ok, err := ask()
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
		Use:         use,
		Short:       short,
		GroupID:     groupID,
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{owned: "group"},
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

func Entry(verb string, names func() []string) func(string) string {
	return func(word string) string {
		for _, n := range names() {
			if n == word {
				return verb + " " + word
			}
		}
		return ""
	}
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

func Answers[T any](values <-chan T, answer func(T) present.Answer) <-chan present.Answer {
	out := make(chan present.Answer)
	go func() {
		defer close(out)
		for v := range values {
			out <- answer(v)
		}
	}()
	return out
}

func Pages[T any](values <-chan T, build func(T) page.Page) <-chan page.Page {
	out := make(chan page.Page)
	go func() {
		defer close(out)
		for v := range values {
			out <- build(v)
		}
	}()
	return out
}
