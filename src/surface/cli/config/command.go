package configcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageconfig "github.com/jterrazz/studio-station/src/pages/config"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the config items placed on this machine, for completion.
	Names func() []string
	Here  page.Context
}

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "config",
		Short:   "The config items that set this machine up",
		Example: "  station config\n  station config --json",
		Group:   kit.OnAMachine,
		Read:    func(*Input) bool { return true },
		Pages:   true,
		Entry:   kit.Entry("station config install", "config item", d.Names),
		Run: func(ctx context.Context, _ *Input, io kit.IO) error {
			reports, err := call.Read(ctx, env.Calls, env.Target(), settingscalls.List, settingscalls.ListIn{}, nil)
			if err != nil {
				return err
			}
			return io.Page(pageconfig.Items(reports, d.Here), pageconfig.ItemsContract(reports))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
