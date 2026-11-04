package wire

import (
	"github.com/spf13/cobra"

	backgroundcalls "github.com/jterrazz/studio-station/src/background/calls"
	"github.com/jterrazz/studio-station/src/background/contract"
	backgroundcmd "github.com/jterrazz/studio-station/src/surface/cli/background"
	backgroundremove "github.com/jterrazz/studio-station/src/surface/cli/background/remove"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func backgroundCommand(env kit.Env, m machine) *cobra.Command {
	return backgroundcmd.Command(backgroundcmd.Deps{Here: m.where}, env,
		backgroundremove.Command(backgroundremove.Deps{Here: m.where}, env))
}

func (m machine) backgroundDeps() backgroundcalls.Deps {
	return backgroundcalls.Deps{
		Jobs: m.jobs(0), Domain: contract.Domain{UID: m.host.Env.Euid()},
		Home: m.host.Env.Home(), Root: m.systemRoot(), Kind: m.profile.Kind,
	}
}
