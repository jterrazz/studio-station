package wire

import (
	"github.com/spf13/cobra"

	activitycmd "github.com/jterrazz/studio-station/src/surface/cli/activity"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func activityCommand(env kit.Env, m machine) *cobra.Command {
	return activitycmd.Command(activitycmd.Deps{Here: m.where, Now: m.host.Clock.Now, Label: m.label}, env)
}
