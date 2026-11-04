package toolsinstall

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/apps"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Names func() []string
	Here  page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "install <tool...>",
		Short:    "Install catalogue tools here, then the config items each one brings",
		Example:  "  station tools install node gh\n  station tools install frontend-design",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := call.Invoke(ctx, env.Calls, env.Target, appscalls.Install, appscalls.InstallIn{Names: in.Names, Interactive: io.Interactive}, nil)
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

func partial(outcomes []apps.Outcome, repair string) error {
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
