package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/ports/adapter/lsof"
	"github.com/jterrazz/studio-station/src/settings/adapter/macos"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	statuscmd "github.com/jterrazz/studio-station/src/surface/cli/status"
)

func statusCommand(env kit.Env, m machine) *cobra.Command {
	return statuscmd.Command(statuscmd.Deps{Status: statusService(env, m), Here: m.where}, env)
}

func statusService(env kit.Env, m machine) *status.Service {
	return status.New(m.host, status.Sources{
		Tools:      toolsService(env, m),
		Config:     configService(m),
		Tailnet:    m.tailnetStatus(),
		Setup:      setupService(env, m),
		Name:       m.stores.name(),
		Identity:   m.stores.identity(),
		Storage:    m.storage(env),
		Daemon:     m.hostDaemon(),
		Power:      statusPower{s: m.power()},
		Hypervisor: m.hypervisor(),
		// value: the status page reads listeners on a Mac only.
		Listeners: lsof.New(m.host.Run),
		Jobs:      m.jobs(status.ReadBudget),
		Inspector: macos.New(m.host.Run, m.host.FS, m.host.Clock, macos.Options{Root: m.systemRoot()}),
	}, status.Options{
		Pool:         m.pool,
		Root:         m.systemRoot(),
		Here:         m.kind,
		OpenClawPort: m.port(defaultOpenClawPort, "OPENCLAW_GATEWAY_PORT"),
		HermesPort:   m.port(defaultHermesPort, "HERMES_GATEWAY_PORT", "API_SERVER_PORT"),
		Version:      Version(),
		Commit:       Commit(),
	})
}
