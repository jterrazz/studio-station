package machinessupervise

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
)

type Deps struct{ Machines *machines.Service }

type Input struct{ Name string }

func Command(d Deps, env kit.Env) *cobra.Command {
	// invariant: plumbing for the VM's launchd agent, which reads its exit code:
	// no answer is printed, and tart's own output goes to the agent's log.
	cmd := kit.Command(kit.Leaf[Input]{
		Use:   "supervise <vm>",
		Short: "Run a VM for its launchd agent, and end a temporary one on its declared poweroff",
		Args:  cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error {
			if err := d.Machines.CheckSupervise(in.Name); err != nil {
				return err
			}
			_, err := d.Machines.VM(in.Name, "supervise")
			return err
		},
		Run: func(ctx context.Context, in *Input, _ kit.IO) error { return d.Machines.Supervise(ctx, in.Name) },
	}, env)
	cmd.Hidden = true
	return cmd
}
