package machinesstart

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
	return kit.Command(kit.Leaf[Input]{
		Use:     "start <vm>",
		Short:   "Start a stopped lasting VM, and keep it starting with its host",
		Example: "  station machines start api",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error { return d.Machines.CheckStart(in.Name) },
		Complete: func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			stop := io.Wait("starting " + in.Name)
			err := d.Machines.Start(ctx, in.Name)
			stop()
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Change("start", in.Name, "started", d.Here), nil)
		},
	}, env)
}
