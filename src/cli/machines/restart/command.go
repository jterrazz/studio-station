package machinesrestart

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
)

type Deps struct {
	Machines *machines.Service
	Here     page.Context
}

type Input struct {
	Name string
	VM   bool
}

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
		Validate: func(_ context.Context, in *Input) error {
			if err := d.Machines.CheckHost(); err != nil {
				return err
			}
			m, err := d.Machines.Remote(in.Name)
			if err != nil {
				return err
			}
			if m.Kind == catalog.Host {
				return nil
			}
			in.VM = true
			if m.Lifetime == machines.Temporary {
				return &machines.TemporaryRestartError{Name: in.Name}
			}
			return nil
		},
		Destructive: func(*Input) bool { return true },
		Confirm: func(in *Input) string {
			if in.VM {
				return "Restart " + in.Name + " (poweroff, then start)?"
			}
			return "Restart " + in.Name + " (sudo fdesetup authrestart)?"
		},
		Complete: func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if _, err := d.Machines.Restart(ctx, in.Name, nil); err != nil {
				return err
			}
			return io.Page(pagemachines.Restarted(in.Name, d.Here), nil)
		},
	}, env)
}
