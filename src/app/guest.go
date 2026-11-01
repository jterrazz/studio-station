package app

import (
	"github.com/spf13/cobra"

	guestcmd "github.com/jterrazz/studio-station/src/cli/guest"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/guest"
)

func (m machine) guest() *guest.Service {
	root := m.systemRoot()
	if root == "" {
		root = "/"
	}
	return guest.New(m.host, guest.Options{Root: root, Paths: m.paths, Kind: m.kind, Version: Version()})
}

func guestCommand(env kit.Env, m machine) *cobra.Command {
	return guestcmd.Command(guestcmd.Deps{Guest: m.guest()}, env)
}
