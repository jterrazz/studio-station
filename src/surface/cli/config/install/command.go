package configinstall

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Names func() []string
	Here  page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:      "install <item...>",
		Short:    "Apply config items here; an item with values asks them in a form on a terminal",
		Example:  "  station config install hushlogin tmux-dotfiles\n  station config install claude-token",
		Args:     cobra.MinimumNArgs(1),
		Bind:     bind,
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			changes, err := call.Invoke(ctx, env.Calls, env.Target(), settingscalls.Install, settingscalls.InstallIn{Names: in.Names, Interactive: io.Interactive}, nil)
			if err != nil {
				return err
			}
			if err := io.Page(pageconfig.Changes("install", changes, d.Here), nil); err != nil {
				return err
			}
			failed, hint := 0, ""
			for _, c := range changes {
				if c.Result == catalog.StateBroken {
					if failed == 0 {
						hint = "re-run it: station config install " + c.Item
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
