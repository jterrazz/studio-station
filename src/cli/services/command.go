package servicescmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/services"
	presentservices "github.com/jterrazz/studio-station/src/present/services"
)

type Deps struct {
	Services *services.Service
	Context  string
}

type Input struct{ Here bool }

func Command(d Deps, env kit.Env, children ...*cobra.Command) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:   "services",
		Short: "What the fleet's VMs listen on, and the URL each is published at",
		Example: "  station services\n  station services --json\n" +
			"  station services url 3000        # in a VM: the public URL, before anything listens\n" +
			"  station services release api:3000",
		Group: "fleet",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.Here, "here", false, "only this host")
		},
		Read: func(*Input) bool { return true },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			fleet, err := d.Services.List(ctx, in.Here)
			if err != nil {
				return err
			}
			return io.Answer(presentservices.Fleet(fleet, d.Context))
		},
	}, env)
	cmd.AddCommand(children...)
	return cmd
}
