package machinesstop

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the machines known here, for completion.
	Names func() []string
	Here  page.Context
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
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			stop := io.Wait("powering " + in.Name + " off")
			_, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.Stop, machinescalls.VMIn{VM: in.Name}, nil)
			stop()
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Change("stop", in.Name, "stopped", d.Here), nil)
		},
	}, env)
}
