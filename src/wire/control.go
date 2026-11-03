package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	startcmd "github.com/jterrazz/studio-station/src/cli/start"
	stopcmd "github.com/jterrazz/studio-station/src/cli/stop"
	"github.com/jterrazz/studio-station/src/domain/daemon"
)

func (m machine) control() *daemon.Control {
	return daemon.NewControl(m.host, daemon.Dial(m.host, m.paths), m.machines(false), daemon.ControlOptions{
		Here: m.kind, HereName: m.where.Machine, State: m.paths.State,
	})
}

func stopCommand(env kit.Env, m machine) *cobra.Command {
	return stopcmd.Command(stopcmd.Deps{Control: m.control(), Here: m.where}, env)
}

func startCommand(env kit.Env, m machine) *cobra.Command {
	return startcmd.Command(startcmd.Deps{Control: m.control(), Here: m.where}, env)
}
