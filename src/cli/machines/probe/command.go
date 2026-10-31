package machinesprobe

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
	presentmachines "github.com/jterrazz/studio-station/src/present/machines"
)

type Deps struct {
	Machines *machines.Service
	Context  string
}

type Input struct{ Name string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "probe <machine>",
		Short:   "Reach a machine without logging in: ping, ssh, the gateway ports, the console owner",
		Example: "  station machines probe mac-mini --json",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error {
			_, err := d.Machines.Remote(in.Name)
			return err
		},
		Read:     func(*Input) bool { return true },
		Complete: func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := d.Machines.Probe(ctx, in.Name)
			if err != nil {
				return err
			}
			return io.Answer(presentmachines.Probe(report, d.Context))
		},
	}, env)
}
