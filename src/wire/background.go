package wire

import (
	"github.com/spf13/cobra"

	backgroundcalls "github.com/jterrazz/studio-station/src/background/calls"
	backgroundcmd "github.com/jterrazz/studio-station/src/surface/cli/background"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func backgroundCommand(env kit.Env, m machine) *cobra.Command {
	return backgroundcmd.Command(backgroundcmd.Deps{Here: m.where}, env)
}

func (m machine) backgroundDeps() backgroundcalls.Deps {
	return backgroundcalls.Deps{
		Jobs: m.jobs(0), Home: m.host.Env.Home(), Root: m.systemRoot(), Kind: m.profile.Kind,
	}
}
