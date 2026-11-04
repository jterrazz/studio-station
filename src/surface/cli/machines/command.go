package machinescmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{ Here bool }

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:     "machines",
		Short:   "Every host and the VMs on each",
		Example: "  station machines\n  station machines --here --json",
		Group:   "fleet",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.Here, "here", false, "this host only, without asking the others")
		},
		Read:  func(*Input) bool { return true },
		Pages: true,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			out, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.List, machinescalls.ListIn{Here: in.Here}, nil)
			if err != nil {
				return err
			}
			var warnings []string
			if out.Unrefreshed != nil {
				warnings = append(warnings, "ssh config not refreshed: "+out.Unrefreshed.Error())
			}
			return io.Page(pagemachines.List(out.Fleet, warnings, d.Here), machines.ToWire(out.Fleet))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
