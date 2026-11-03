package configinstall

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Config *config.Service
	Here   page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "install <item...>",
		Short:   "Apply config items here; an item with values asks them in a form on a terminal",
		Example: "  station config install hushlogin tmux-dotfiles\n  station config install claude-token",
		Args:    cobra.MinimumNArgs(1),
		Bind:    bind,
		Complete: func([]string, string) []string {
			placed := d.Config.Placed()
			out := make([]string, 0, len(placed))
			for _, it := range placed {
				out = append(out, it.Name)
			}
			return out
		},
		Validate: func(_ context.Context, in *Input) error { return d.Config.CheckInstall(in.Names, env.Interactive) },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			items, err := d.Config.Find(in.Names)
			if err != nil {
				return err
			}
			changes := make([]config.Change, 0, len(items))
			for _, it := range items {
				values, err := ask(d, it, io)
				if err != nil {
					return err
				}
				change, _ := d.Config.Install(ctx, it.Name, values, io.Interactive)
				changes = append(changes, change)
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

func ask(d Deps, it catalog.ConfigItem, io kit.IO) (map[string]string, error) {
	if len(it.Inputs) == 0 || !io.Interactive {
		return nil, nil
	}
	return io.Prompt.Form(pageconfig.InputsForm(it.Name, it.Inputs, d.Config.Defaults(it.Name)))
}
