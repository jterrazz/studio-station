package toolsuninstall

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tools"
	presenttools "github.com/jterrazz/studio-station/src/present/tools"
)

type Deps struct {
	Tools   *tools.Service
	Context string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:         "uninstall <tool...>",
		Short:       "Uninstall catalogue tools from here, through the method that installed them",
		Example:     "  station tools uninstall bat --yes",
		Args:        cobra.MinimumNArgs(1),
		Bind:        bind,
		Validate:    func(_ context.Context, in *Input) error { return d.Tools.CheckUninstall(in.Names) },
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Uninstall " + strings.Join(in.Names, ", ") + "?" },
		Complete: func([]string, string) []string {
			placed := d.Tools.Placed()
			out := make([]string, 0, len(placed))
			for _, t := range placed {
				out = append(out, t.Name)
			}
			return out
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := d.Tools.Uninstall(ctx, in.Names)
			if err != nil {
				return err
			}
			if err := io.Answer(presenttools.Outcomes("uninstall", outcomes, d.Context)); err != nil {
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
