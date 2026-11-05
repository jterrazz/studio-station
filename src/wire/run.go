package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	settingscmd "github.com/jterrazz/studio-station/src/surface/cli/settings"
)

func runCommand(env kit.Env, m machine) *cobra.Command {
	return settingscmd.Run(env, settingscmd.RunGit(env), settingscmd.RunContainers(env, m.where))
}
