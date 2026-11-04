package machinesrestart

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
	Names func() []string
	// value: whether a name is a VM, for the question.
	IsVM func(name string) bool
	Here page.Context
}

type Input struct{ Name string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "restart <machine>",
		Short:   "Restart a host with a FileVault-aware reboot, or a lasting VM with a poweroff and a start",
		Example: "  station machines restart mac-mini --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Asks: true,
		Confirm: func(in *Input) string {
			if d.IsVM(in.Name) {
				return "Restart " + in.Name + " (poweroff, then start)?"
			}
			return "Restart " + in.Name + " (sudo fdesetup authrestart)?"
		},
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			err := io.Confirmed(func(answer string) error {
				_, err := call.Invoke(ctx, env.Calls, env.Target(), machinescalls.Restart, machinescalls.RestartIn{Name: in.Name, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Restarted(in.Name, d.Here), nil)
		},
	}, env)
}
