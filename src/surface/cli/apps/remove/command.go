package appsremove

import (
	"context"
	"strings"

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
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "remove <app...>",
		Short:    "Remove apps from here, through the method that installed them; their data stays",
		Example:  "  station apps remove lazygit\n  station apps remove bat --yes",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Asks:     true,
		Confirm:  func(in *Input) string { return "Remove " + strings.Join(in.Names, ", ") + "?" },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var outcomes []apps.Outcome
			err := io.Confirmed(func(answer string) (err error) {
				outcomes, err = call.Invoke(ctx, env.Calls, env.Target(), appscalls.Remove, appscalls.RemoveIn{Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pageapps.Outcomes("remove", outcomes, d.Here), nil); err != nil {
				return err
			}
			return partial(outcomes, "re-run it: station apps remove ")
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
