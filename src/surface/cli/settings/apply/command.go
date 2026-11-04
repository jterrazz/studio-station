package settingsapply

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Names func() []string
	Here  page.Context
}

type Input struct{ Names []string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "apply [<setting...>]",
		Short:    "Apply settings here; bare, every one not applied or changed since; one with values asks them on a terminal",
		Example:  "  station settings apply\n  station settings apply starship-dotfiles\n  station settings apply claude-token",
		Args:     cobra.ArbitraryArgs,
		Bind:     func(in *Input, args []string) error { in.Names = args; return nil },
		Asks:     true,
		Confirm:  func(in *Input) string { return pagesettings.ApplyConfirm(in.Names).Question },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var changes []settings.Change
			err := io.Confirmed(func(answer string) (err error) {
				changes, err = call.Invoke(ctx, env.Calls, env.Target(), settingscalls.Apply, settingscalls.ApplyIn{Names: in.Names, Interactive: io.Interactive, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if err := io.Page(pagesettings.Changes("apply", changes, d.Here), nil); err != nil {
				return err
			}
			return partial(changes, "apply")
		},
	}, env)
}

func partial(changes []settings.Change, verb string) error {
	failed, hint := 0, ""
	for _, c := range changes {
		if c.Result == catalog.StateBroken {
			if failed == 0 {
				hint = "re-run it: station settings " + verb + " " + c.Item
			}
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return kit.PartialFailure{Failed: failed, Total: len(changes), Noun: "settings", FirstHint: hint}
}
