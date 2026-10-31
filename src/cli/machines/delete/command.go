package machinesdelete

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
		Use:     "delete <vm>",
		Short:   "Destroy a VM: its disk, its agent, its tailnet node and its lent copies",
		Example: "  station machines delete linux-1 --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error {
			_, err := d.Machines.VM(in.Name, "delete")
			return err
		},
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Delete " + in.Name + " and everything on its disk?" },
		Complete:    func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			stop := io.Wait("deleting " + in.Name)
			report, err := d.Machines.Delete(ctx, in.Name)
			stop()
			if err != nil {
				return err
			}
			return io.Answer(presentmachines.Deleted(report, d.Context))
		},
	}, env)
}
