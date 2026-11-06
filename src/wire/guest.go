package wire

import (
	"context"
	"io"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/ports/adapter/docker"
	"github.com/jterrazz/studio-station/src/ports/adapter/guest"
	"github.com/jterrazz/studio-station/src/ports/adapter/lsof"
	"github.com/jterrazz/studio-station/src/ports/adapter/procnet"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	guestcmd "github.com/jterrazz/studio-station/src/surface/cli/guest"
	guestserve "github.com/jterrazz/studio-station/src/surface/cli/guest/serve"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) guest() *guest.Service {
	root := m.systemRoot()
	if root == "" {
		root = "/"
	}
	return guest.New(m.sys, guest.Options{State: m.paths.State, ServicesEnv: filepath.Join(m.paths.Config, "services.env"), Kind: m.kind, Version: Version(), Listeners: m.listeners(root), Containers: m.containers(), Accounts: func(ctx context.Context, keys []string) any {
		a := accountscalls.NewReader(m.pool, m.identities(path.Join(m.paths.State, "children")), accountscalls.ReaderPorts{}, accountscalls.ReaderOptions{ThisMachine: m.kind, ThisMacName: m.where.Machine, Now: m.sys.Clock.Now, Keys: func() []string { return keys }})
		return a.Own(ctx)
	}})
}

// value: the ports this VM's containers publish; none in a world.
func (m machine) containers() guest.Containers {
	if m.world != nil {
		return nil
	}
	return docker.New(m.sys.Run)
}

// value: station guest, refused on a Mac.
type guestServer struct {
	*guest.Service
	kind catalog.MachineKind
}

func (g guestServer) Check() error {
	if g.kind == catalog.Mac {
		return &model.GuestOnMacError{}
	}
	return nil
}

func (g guestServer) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	if err := g.Check(); err != nil {
		return err
	}
	return g.Service.Run(ctx, in, out)
}

// value: what listens here, read the way this system can: a Linux kernel's
// /proc/net tables under root, a Mac's lsof.
func (m machine) listeners(root string) portscontract.Listeners {
	if m.world != nil {
		return m.world.listeners()
	}
	if m.kind == catalog.Linux {
		return procnet.New(m.sys.FS, root)
	}
	return lsof.New(m.sys.Run)
}

func guestCommand(env kit.Env, m machine) *cobra.Command {
	cmd := guestcmd.Command(guestcmd.Deps{Guest: guestServer{m.guest(), m.kind}}, env)
	cmd.AddCommand(guestserve.Command(guestserve.Deps{Serve: serveFrames(env)}, env))
	return cmd
}
