package machinesforget

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Machines *machines.Service
	Here     page.Context
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
		Validate: func(_ context.Context, in *Input) error {
			if err := d.Machines.CheckHost(); err != nil {
				return err
			}
			_, err := d.Machines.RemoteHost(in.Name, "forget")
			return err
		},
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Forget " + in.Name + "?" },
		Complete:    func([]string, string) []string { return d.Machines.Names() },
		Run: func(_ context.Context, in *Input, io kit.IO) error {
			if err := d.Machines.ForgetHost(in.Name); err != nil {
				return err
			}
			return io.Page(pagemachines.Change("forget", in.Name, "forgotten", d.Here), nil)
		},
	}, env)
}
