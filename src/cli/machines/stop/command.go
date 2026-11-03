package machinesstop

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
)

type Deps struct {
	Machines *machines.Service
	Here     page.Context
}

type Input struct{ Name string }

func Command(d Deps, env kit.Env) *cobra.Command {
	// invariant: stop never destroys, so it asks nothing and has no --yes; a
	// temporary VM is refused, since its stop would be its end.
	return kit.Command(kit.Leaf[Input]{
		Use:     "stop <vm>",
		Short:   "Power a lasting VM off; it stays off until started",
		Example: "  station machines stop api",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error { return d.Machines.CheckStop(in.Name) },
		Complete: func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			stop := io.Wait("powering " + in.Name + " off")
			err := d.Machines.Stop(ctx, in.Name)
			stop()
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Change("stop", in.Name, "stopped", d.Here), nil)
		},
	}, env)
}
