package machinesdelete

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/machines"
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
	return kit.Command(kit.Leaf[Input]{
		Use:     "delete <vm>",
		Short:   "Destroy a VM: its disk, its agent, its tailnet node and its lent copies",
		Example: "  station machines delete linux-1 --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Destructive: func(*Input) bool { return true },
		ByCall:      true,
		Confirm:     func(in *Input) string { return "Delete " + in.Name + " and everything on its disk?" },
		Complete:    func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var report machines.DeleteReport
			err := io.Confirmed(func(answer string) (err error) {
				if answer == in.Name {
					stop := io.Wait("deleting " + in.Name)
					defer stop()
				}
				report, err = call.Invoke(ctx, env.Calls, env.Target, machinescalls.Delete, machinescalls.DeleteIn{VM: in.Name, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Deleted(report, d.Here), nil)
		},
	}, env)
}
