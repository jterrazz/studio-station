package wire

import (
	"context"
	"path"

	"github.com/spf13/cobra"

	guestcmd "github.com/jterrazz/studio-station/src/cli/guest"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/domain/guest"
)

func (m machine) guest() *guest.Service {
	root := m.systemRoot()
	if root == "" {
		root = "/"
	}
	return guest.New(m.host, guest.Options{Root: root, Paths: m.paths, Kind: m.kind, Version: Version(), Accounts: func(ctx context.Context, keys []string) any {
		a := accounts.New(m.host, m.pool, accounts.Ports{}, accounts.Options{Here: m.kind, HereName: m.where.Machine, Track: path.Join(m.paths.State, "children"), Keys: func() []string { return keys }})
		return a.Own(ctx)
	}})
}

func guestCommand(env kit.Env, m machine) *cobra.Command {
	return guestcmd.Command(guestcmd.Deps{Guest: m.guest()}, env)
}
