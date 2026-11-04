package appsinstall

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/apps"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageapps "github.com/jterrazz/studio-station/src/pages/apps"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Names func() []string
	Here  page.Context
	Now   func() time.Time
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "install <app...>",
		Short:    "Install apps here, in dependency order, then the settings each one brings",
		Example:  "  station apps install jq fd\n  station apps install frontend-design",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			start := d.Now()
			steps := kit.Steps(io)
			outcomes, err := call.Invoke(ctx, env.Calls, env.Target(), appscalls.Install, appscalls.InstallIn{Names: in.Names, Interactive: io.Interactive}, steps)
			steps.Stop()
			if err != nil {
				return err
			}
			if err := io.Page(pageapps.Outcomes("install", outcomes, ran(env, d.Now().Sub(start)), d.Here), nil); err != nil {
				return err
			}
			return partial(outcomes, "station apps install ")
		},
	}, env)
}

func partial(outcomes []apps.Outcome, repair string) error {
	n, first := apps.FailedOf(outcomes)
	if n == 0 {
		return nil
	}
	return kit.PartialFailure{Failed: n, Total: len(outcomes), Noun: "apps", FirstHint: repair + first}
}

func ran(env kit.Env, took time.Duration) pageapps.Run {
	run := pageapps.Run{Took: took}
	if env.Far() {
		run.On = env.Target().Name
	}
	return run
}
