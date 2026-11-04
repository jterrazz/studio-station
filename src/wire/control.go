package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	startcmd "github.com/jterrazz/studio-station/src/surface/cli/start"
	stopcmd "github.com/jterrazz/studio-station/src/surface/cli/stop"
)

func (m machine) control() *engine.Control {
	return engine.NewControl(m.host, engine.Dial(m.host, m.paths), m.machines(false), engineHypervisor{m.hypervisor()}, m.engineJob(), engine.ControlOptions{
		Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, State: m.paths.State,
	})
}

func stopCommand(env kit.Env, m machine) *cobra.Command {
	return stopcmd.Command(stopcmd.Deps{Control: m.control(), Here: m.where}, env)
}

func startCommand(env kit.Env, m machine) *cobra.Command {
	return startcmd.Command(startcmd.Deps{Control: m.control(), Here: m.where}, env)
}
