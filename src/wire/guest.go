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
	guestcmd "github.com/jterrazz/studio-station/src/surface/cli/guest"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) guest() *guest.Service {
	root := m.systemRoot()
	if root == "" {
		root = "/"
	}
	return guest.New(m.host, guest.Options{State: m.paths.State, ServicesEnv: filepath.Join(m.paths.Config, "services.env"), Kind: m.kind, Version: Version(), Listeners: m.listeners(root), Accounts: func(ctx context.Context, keys []string) any {
		a := accounts.New(m.host, m.pool, m.identities(path.Join(m.paths.State, "children")), accounts.Ports{}, accounts.Options{Here: m.kind, HereName: m.where.Machine, Keys: func() []string { return keys }})
		return a.Own(ctx)
	}})
}

func (m machine) listeners(root string) guest.Listeners {
	if m.kind == catalog.MacOS {
		return lsof.New(m.host.Run)
	}
	return procnet.New(m.host.FS, root)
}

func guestCommand(env kit.Env, m machine) *cobra.Command {
	return guestcmd.Command(guestcmd.Deps{Guest: m.guest()}, env)
}
