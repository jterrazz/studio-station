package configuninstall

import (
	"context"
	"strings"

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
		Use:         "uninstall <item...>",
		Short:       "Undo reversible config items here",
		Example:     "  station config uninstall claude-token --yes",
		Args:        cobra.MinimumNArgs(1),
		Bind:        bind,
		Validate:    func(_ context.Context, in *Input) error { return d.Config.CheckUninstall(in.Names) },
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Uninstall " + strings.Join(in.Names, ", ") + "?" },
		Complete: func([]string, string) []string {
			var out []string
			for _, it := range d.Config.Placed() {
				if it.Reversible {
					out = append(out, it.Name)
				}
			}
			return out
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			items, err := d.Config.Find(in.Names)
			if err != nil {
				return err
			}
			changes := make([]config.Change, 0, len(items))
			for _, it := range items {
				change, _ := d.Config.Uninstall(ctx, it.Name)
				changes = append(changes, change)
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
