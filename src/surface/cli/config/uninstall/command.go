package configuninstall

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the reversible config items placed on this machine.
	Reversible func() []string
	Here       page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "uninstall <item...>",
		Short:    "Undo reversible config items here",
		Example:  "  station config uninstall claude-token --yes",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Asks:     true,
		Confirm:  func(in *Input) string { return "Uninstall " + strings.Join(in.Names, ", ") + "?" },
		Complete: func([]string, string) []string { return d.Reversible() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var changes []settings.Change
			err := io.Confirmed(func(answer string) (err error) {
				changes, err = call.Invoke(ctx, env.Calls, env.Target(), settingscalls.Uninstall, settingscalls.UninstallIn{Names: in.Names, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pageconfig.Changes("uninstall", changes, d.Here), nil); err != nil {
				return err
			}
			failed, hint := 0, ""
			for _, c := range changes {
				if c.Result == catalog.StateBroken {
					if failed == 0 {
						hint = "re-run it: station config uninstall " + c.Item
					}
					failed++
				}
			}
			if failed == 0 {
				return nil
			}
			return kit.PartialFailure{Failed: failed, Total: len(changes), Noun: "config items", FirstHint: hint}
		},
	}, env)
}
