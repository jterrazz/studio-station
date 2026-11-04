package tailnetcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/page"
	pagetailnet "github.com/jterrazz/studio-station/src/pages/tailnet"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Reader interface {
	Status(ctx context.Context) (ports.Status, error)
}

type Deps struct {
	Tailnet Reader
	Here    page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[struct{}]{
		Use:     "tailnet",
		Short:   "This host's node in the Tailscale app, its peers and what it serves",
		Example: "  station tailnet\n  station tailnet --json",
		Group:   "fleet",
		Read:    func(*struct{}) bool { return true },
		Pages:   true,
		Run: func(ctx context.Context, _ *struct{}, io kit.IO) error {
			status, err := d.Tailnet.Status(ctx)
			if err != nil {
				return err
			}
			return io.Page(pagetailnet.Status(status, d.Here), pagetailnet.StatusContract(status))
		},
	}, env)
}
