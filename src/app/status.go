package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	statuscmd "github.com/jterrazz/studio-station/src/cli/status"
	"github.com/jterrazz/studio-station/src/domain/status"
)

func statusCommand(env kit.Env, m machine) *cobra.Command {
	return statuscmd.Command(statuscmd.Deps{Status: statusService(env, m), Context: m.context}, env)
}

func statusService(env kit.Env, m machine) *status.Service {
	return status.New(m.host, status.Sources{
		Tools:   toolsService(env, m),
		Config:  configService(m),
		Tailnet: m.tailnet(),
		Setup:   setupService(env, m),
		Store:   m.store,
		Storage: m.storage(env),
		Daemon:  m.hostDaemon(),
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
