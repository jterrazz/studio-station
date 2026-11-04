package toolsupgrade

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
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
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcomes, err := call.Invoke(ctx, env.Calls, env.Target(), appscalls.Upgrade, appscalls.UpgradeIn{Names: in.Names, Via: managers(in.Via)}, nil)
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
