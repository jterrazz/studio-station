package servicesrelease

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/page"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
)

type Deps struct {
	Services *services.Service
	Here     page.Context
}

type Input struct {
	Text   string
	Target services.Target
	Host   string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "release <vm>:<port> | <port>",
		Short:   "Give a service's host port back: its URL changes at its next listen",
		Example: "  station services release api:3000\n  station services release 3000      # in a VM",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Text = args[0]
			return nil
		},
		Validate: func(ctx context.Context, in *Input) error {
			t, err := d.Services.ParseTarget(in.Text)
			if err != nil {
				return err
			}
			in.Target = t
			in.Host, err = d.Services.CheckRelease(ctx, t)
			return err
		},
		Destructive: func(in *Input) bool { return d.Services.Pinned(in.Target) },
		Confirm: func(in *Input) string {
			return "release " + in.Target.VM + ":" + strconv.Itoa(in.Target.Port) + "? its URL is in the VM's environment"
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			l, err := d.Services.Release(ctx, in.Target, in.Host, io.Yes)
			if err != nil || in.Host != "" {
				return err
			}
			return io.Page(pageservices.Released(l, d.Here), nil)
		},
	}, env)
}
