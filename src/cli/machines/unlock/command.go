package machinesunlock

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
)

type Deps struct {
	Machines *machines.Service
	Context  string
}

type Input struct{ Name string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "unlock <host>",
		Short:   "Open the FileVault pre-boot ssh session of a host, to type its password",
		Example: "  station machines unlock mac-mini",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Name = args[0]
			return nil
		},
		Validate: func(_ context.Context, in *Input) error {
			if err := d.Machines.CheckHost(); err != nil {
				return err
			}
			if _, err := d.Machines.RemoteHost(in.Name, "unlock"); err != nil {
				return err
			}
			if !env.Interactive {
				return &machines.NeedsTerminalError{Verb: "unlock"}
			}
			return nil
		},
		Complete: func([]string, string) []string { return d.Machines.Names() },
		Run: func(ctx context.Context, in *Input, _ kit.IO) error {
			return d.Machines.Unlock(ctx, in.Name)
		},
	}, env)
}
