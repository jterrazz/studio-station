package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	secretscmd "github.com/jterrazz/studio-station/src/surface/cli/secrets"
	secretsallow "github.com/jterrazz/studio-station/src/surface/cli/secrets/allow"
	secretsdisallow "github.com/jterrazz/studio-station/src/surface/cli/secrets/disallow"
	secretslend "github.com/jterrazz/studio-station/src/surface/cli/secrets/lend"
	secretsrecall "github.com/jterrazz/studio-station/src/surface/cli/secrets/recall"
)

func secretsCommand(env kit.Env, m machine) *cobra.Command {
	service := m.secrets()
	return secretscmd.Command(secretscmd.Deps{Secrets: service, Here: m.where}, env,
		secretsallow.Command(secretsallow.Deps{Secrets: service, Here: m.where}, env),
		secretsdisallow.Command(secretsdisallow.Deps{Secrets: service, Here: m.where}, env),
		secretslend.Command(secretslend.Deps{Secrets: service, Here: m.where}, env),
		secretsrecall.Command(secretsrecall.Deps{Secrets: service, Here: m.where}, env),
	)
}

func (m machine) secretStores() secrets.Stores {
	return secrets.Stores{
		ShareList: m.stores.shareList(), Hosts: m.stores.hosts(), Name: m.stores.name(),
		Shares: m.stores.shares, Machines: m.stores.machines,
	}
}
