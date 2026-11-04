package machinesunlock

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Names func() []string
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
		Complete: func([]string, string) []string { return d.Names() },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			_, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.Unlock, machinescalls.UnlockIn{Name: in.Name, Interactive: io.Interactive}, nil)
			return err
		},
	}, env)
}
