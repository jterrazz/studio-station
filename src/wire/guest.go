package wire

import (
	"context"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/domain/guest"
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
	return guest.New(m.sys, guest.Options{State: m.paths.State, ServicesEnv: filepath.Join(m.paths.Config, "services.env"), Kind: m.kind, Version: Version(), Listeners: m.listeners(root), Accounts: func(ctx context.Context, keys []string) any {
		a := accounts.New(m.sys, m.pool, m.identities(path.Join(m.paths.State, "children")), accounts.Ports{}, accounts.Options{Here: m.kind, HereName: m.where.Machine, Keys: func() []string { return keys }})
		return a.Own(ctx)
	}})
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
	cmd := guestcmd.Command(guestcmd.Deps{Guest: m.guest()}, env)
	cmd.AddCommand(guestserve.Command(guestserve.Deps{Serve: serveFrames(env)}, env))
	return cmd
}
