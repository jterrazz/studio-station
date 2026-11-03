package servicesurl

import (
	"context"

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
	Text string
	Port int
}

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
		Validate: func(_ context.Context, in *Input) error {
			port, err := d.Services.CheckURL(in.Text)
			in.Port = port
			return err
		},
		Read: func(*Input) bool { return false },
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			v, err := d.Services.URL(ctx, in.Port)
			if err != nil {
				return err
			}
			return io.PagePiped(pageservices.URL(v, d.Here), v, v.URLs.Public+"\n")
		},
	}, env)
}
