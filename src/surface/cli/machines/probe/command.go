package machinesprobe

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
		Use:     "probe <machine>",
		Short:   "Reach a machine without logging in: ping, ssh, the gateway ports, the console owner",
		Example: "  station machines probe mac-mini --json",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Read:     func(*Input) bool { return true },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			report, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.Probe, machinescalls.ProbeIn{Name: in.Name}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Probe(report, d.Here), pagemachines.ProbeContract(report))
		},
	}, env)
}
