package cleancmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/clean"
	presentclean "github.com/jterrazz/studio-station/src/present/clean"
)

type Deps struct {
	Clean   *clean.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	var flags *pflag.FlagSet
	return kit.Command(kit.Leaf[Input]{
		Use:       "clean [<cleanable...> | --all]",
		Short:     "What can be reclaimed, with its size; named or --all, reclaim it",
		Example:   "  station clean\n  station clean brew trash --yes\n  station clean --all --yes",
		Args:      cobra.ArbitraryArgs,
		ValidArgs: names(d.Clean),
		Declare: func(f *pflag.FlagSet, in *Input) {
			flags = f
			f.BoolVarP(&in.All, "all", "a", false, "reclaim every cleanable placed here")
		},
		Bind: bind,
		Validate: func(_ context.Context, in *Input) error {
			asked := func(flag string) bool { v, _ := flags.GetBool(flag); return v }
			switch {
			case !in.acts() && asked("yes"):
				return kit.Refuse("--yes has nothing to confirm", "station clean <item...> --yes")
			case in.acts() && asked("json"):
				return kit.Refuse("--json lists; it cannot reclaim", "station clean --json")
			}
			_, err := d.Clean.Find(in.Names)
			return err
		},
		Read:        func(in *Input) bool { return !in.acts() },
		Destructive: func(in *Input) bool { return in.acts() },
		Confirm: func(in *Input) string {
			if in.All {
				return "reclaim everything listed?"
			}
			return "reclaim the named cleanables?"
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if !in.acts() {
				reports, err := d.Clean.List(ctx)
				if err != nil {
					return err
				}
				return io.Answer(presentclean.List(reports, d.Context))
			}
			targets := in.Names
			if in.All {
				targets = names(d.Clean)
			}
			outcomes, err := d.Clean.Clean(ctx, targets)
			if err != nil {
				return err
			}
			if err := io.Answer(presentclean.Outcomes(outcomes, d.Context)); err != nil {
				return err
			}
			return partial(outcomes)
		},
	}, env)
}

func names(s *clean.Service) []string {
	placed := s.Placed()
	out := make([]string, 0, len(placed))
	for _, c := range placed {
		out = append(out, c.Name)
	}
	return out
}

func partial(outcomes []clean.Outcome) error {
	failed, hint := 0, ""
	for _, o := range outcomes {
		if o.Result == catalog.StateBroken {
			if failed == 0 {
				hint = "re-run it: station clean " + o.Name + " --yes"
			}
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "cleanables", FirstHint: hint}
}
