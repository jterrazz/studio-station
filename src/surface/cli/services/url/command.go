package servicesurl

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
}

type Input struct{ Text string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "url <port>",
		Short:   "In a VM: reserve a port's public URL now, and print it",
		Example: "  API_URL=$(station services url 3000) pnpm dev",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Text = args[0]
			return nil
		},
		Read: func(*Input) bool { return false },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			v, err := call.Invoke(ctx, env.Calls, env.Target, portscalls.URL, portscalls.URLIn{Port: in.Text}, nil)
			if err != nil {
				return err
			}
			return io.PagePiped(pageservices.URL(v, d.Here), v, v.URLs.Public+"\n")
		},
	}, env)
}
