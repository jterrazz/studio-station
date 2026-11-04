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
	return kit.Command(kit.Leaf[Input]{
		Use:     "stop <vm>",
		Short:   "Stop a workspace; it is kept, with its disk, until started or deleted",
		Example: "  station machines stop linux-1 --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Asks:     true,
		Confirm:  func(in *Input) string { return "Stop " + in.Name + "? It is kept, stopped, until you start it" },
		Verb:     func(*Input) string { return "stop" },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			already := false
			err := io.Confirmed(func(answer string) error {
				already = answer == ""
				stop := io.Wait("stopping " + in.Name)
				defer stop()
				_, err := call.Invoke(ctx, env.Calls, env.Target(), machinescalls.Stop, machinescalls.StopIn{VM: in.Name, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			if already {
				return io.Page(pagemachines.Already("stop", in.Name, false, d.Here), nil)
			}
			return io.Page(pagemachines.Change("stop", in.Name, "stopped, kept", d.Here), nil)
		},
	}, env)
}
