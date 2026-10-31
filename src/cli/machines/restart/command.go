package machinesrestart

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
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
		Use:     "restart <machine>",
		Short:   "Restart a host with a FileVault-aware reboot, and wait for it to come back",
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
			switch {
			case m.Kind == catalog.Host:
				return nil
			case m.Lifetime == machines.Temporary:
				return &machines.TemporaryRestartError{Name: in.Name}
			}
			return kit.NotBuilt()
		},
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "Restart " + in.Name + " (sudo fdesetup authrestart)?" },
		Complete:    func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var events []machines.RestartEvent
			report, err := d.Machines.Restart(ctx, in.Name, func(e machines.RestartEvent) { events = append(events, e) })
			if err != nil {
				return err
			}
			return io.Answer(presentmachines.Restart(events, report, d.Context))
		},
	}, env)
}
