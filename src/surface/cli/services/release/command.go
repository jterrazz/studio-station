package servicesrelease

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	"github.com/jterrazz/studio-station/src/ports"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the VM and port a target names, for the question.
	Target func(text string) (ports.Target, error)
	Here   page.Context
}

type Input struct{ Text string }

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
		Destructive: func(*Input) bool { return true },
		ByCall:      true,
		Confirm: func(in *Input) string {
			t, _ := d.Target(in.Text)
			return "release " + t.VM + ":" + strconv.Itoa(t.Port) + "? its URL is in the VM's environment"
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			var out portscalls.ReleaseOut
			err := io.Confirmed(func(answer string) (err error) {
				out, err = call.Invoke(ctx, env.Calls, env.Target, portscalls.Release, portscalls.ReleaseIn{Target: in.Text, Confirm: answer}, nil)
				return err
			})
			if err != nil || out.Host != "" {
				return err
			}
			return io.Page(pageservices.Released(out.Lease, d.Here), nil)
		},
	}, env)
}
