package machinesforget

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
		Use:     "forget <host>",
		Short:   "Forget a host: its registration and ssh block go, the host is not touched",
		Example: "  station machines forget mac-mini --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Asks:     true,
		Confirm:  func(in *Input) string { return "Forget " + in.Name + "?" },
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			err := io.Confirmed(func(answer string) error {
				_, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.Forget, machinescalls.ForgetIn{Name: in.Name, Confirm: answer}, nil)
				return err
			})
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Change("forget", in.Name, "forgotten", d.Here), nil)
		},
	}, env)
}
