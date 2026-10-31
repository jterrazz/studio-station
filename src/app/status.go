package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli/kit"
	statuscmd "github.com/jterrazz/studio-station/src/cli/status"
	"github.com/jterrazz/studio-station/src/domain/status"
)

func statusCommand(env kit.Env, m machine) *cobra.Command {
	service := status.New(m.host, status.Sources{
		Tools:   toolsService(env, m),
		Config:  configService(m),
		Tailnet: m.tailnet(env.Interactive),
		Setup:   setupService(env, m),
		Store:   m.store,
	}, status.Options{
		Here:         m.kind,
		OpenClawPort: m.port(defaultOpenClawPort, "OPENCLAW_GATEWAY_PORT"),
		HermesPort:   m.port(defaultHermesPort, "HERMES_GATEWAY_PORT", "API_SERVER_PORT"),
		Version:      Version(),
		Commit:       Commit(),
	})
	return statuscmd.Command(statuscmd.Deps{Status: service, Context: m.context}, env)
}
