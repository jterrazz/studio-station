package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	runcmd "github.com/jterrazz/studio-station/src/surface/cli/run"
	"github.com/jterrazz/studio-station/src/surface/cli/run/docker"
	settingscmd "github.com/jterrazz/studio-station/src/surface/cli/settings"
)

func runCommand(env kit.Env, m machine) *cobra.Command {
	return runcmd.Command(env,
		settingscmd.RunGit(env),
		rundocker.Command(rundocker.Deps{Here: m.where}, env),
	)
}
