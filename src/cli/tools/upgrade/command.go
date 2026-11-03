package toolsupgrade

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
)

type Deps struct {
	Tools *tools.Service
	Here  page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "upgrade [<tool...> | --via <manager>...]",
		Short:   "Upgrade installed catalogue tools; bare, every one of them",
		Example: "  station tools upgrade\n  station tools upgrade node gh\n  station tools upgrade --via " + strings.Join(managerNames(), " --via "),
		Args:    cobra.ArbitraryArgs,
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringArrayVar(&in.Via, "via", nil, "upgrade every installed tool of a manager: "+strings.Join(managerNames(), ", "))
		},
		Bind: bind,
		Validate: func(_ context.Context, in *Input) error {
			if len(in.Names) == 0 {
				return nil
			}
			return d.Tools.CheckUpgrade(in.Names)
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := d.Tools.Upgrade(ctx, tools.UpgradeRequest{Names: in.Names, All: len(in.Names) == 0 && len(in.Via) == 0, Via: managers(in.Via)})
			if err != nil {
				return err
			}
			if err := io.Page(pagetools.Outcomes("upgrade", outcomes, d.Here), nil); err != nil {
				return err
			}
			failed, hint := 0, ""
			for _, o := range outcomes {
				if o.Result == catalog.StateBroken {
					if failed == 0 {
						hint = "re-run it: station tools upgrade " + o.Name
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
