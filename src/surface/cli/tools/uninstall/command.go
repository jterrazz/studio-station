package toolsuninstall

import (
	"context"
	"strings"

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
		Use:      "uninstall <tool...>",
		Short:    "Uninstall catalogue tools from here, through the method that installed them",
		Example:  "  station tools uninstall bat --yes",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Asks:     true,
		Confirm:  func(in *Input) string { return "Uninstall " + strings.Join(in.Names, ", ") + "?" },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var outcomes []apps.Outcome
			err := io.Confirmed(func(answer string) (err error) {
				outcomes, err = call.Invoke(ctx, env.Calls, env.Target, appscalls.Uninstall, appscalls.UninstallIn{Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pagetools.Outcomes("uninstall", outcomes, d.Here), nil); err != nil {
				return err
			}
			failed, hint := 0, ""
			for _, o := range outcomes {
				if o.Result == catalog.StateBroken {
					if failed == 0 {
						hint = "re-run it: station tools uninstall " + o.Name
					}
					failed++
				}
			}
			if failed > 0 {
				return kit.PartialFailure{Failed: failed, Total: len(outcomes), Noun: "tools", FirstHint: hint}
			}
			return nil
		},
	}, env)
}
