package machinescmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
)

type Deps struct {
	Machines *machines.Service
	Here     page.Context
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
			fleet, err := d.Machines.Fleet(ctx, in.Here)
			if err != nil {
				return err
			}
			var warnings []string
			if !in.Here {
				if refreshErr := d.Machines.SSHBlocks(ctx, fleet); refreshErr != nil {
					warnings = append(warnings, "ssh config not refreshed: "+refreshErr.Error())
				}
			}
			return io.Page(pagemachines.List(fleet, warnings, d.Here), machines.ToWire(fleet))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
