package toolsinstall

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
)

type Deps struct {
	Tools *tools.Service
	Here  page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "install <tool...>",
		Short:    "Install catalogue tools here, then the config items each one brings",
		Example:  "  station tools install node gh\n  station tools install frontend-design",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Complete: names(d.Tools),
		Validate: func(ctx context.Context, in *Input) error { return d.Tools.CheckInstall(ctx, in.Names) },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := d.Tools.Install(ctx, tools.InstallRequest{Names: in.Names, Interactive: io.Interactive})
			if err != nil {
				return err
			}
			if err := io.Page(pagetools.Outcomes("install", outcomes, d.Here), nil); err != nil {
				return err
			}
			return partial(outcomes, "station tools install ")
		},
	}, env)
}

func names(s *tools.Service) func([]string, string) []string {
	return func([]string, string) []string {
		placed := s.Placed()
		out := make([]string, 0, len(placed))
		for _, t := range placed {
			out = append(out, t.Name)
		}
		return out
	}
}

func partial(outcomes []tools.Outcome, repair string) error {
	failed, hint := 0, ""
	for _, o := range outcomes {
		if o.Result == catalog.StateBroken {
			if failed == 0 {
				hint = repair + o.Name
			}
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "tools", FirstHint: hint}
}
