package wire

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/engine"
	accountscmd "github.com/jterrazz/studio-station/src/surface/cli/accounts"
	accountsshare "github.com/jterrazz/studio-station/src/surface/cli/accounts/share"
	accountstakeback "github.com/jterrazz/studio-station/src/surface/cli/accounts/takeback"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func accountsCommand(env kit.Env, m machine) *cobra.Command {
	return accountscmd.Command(accountscmd.Deps{Here: m.where}, env,
		accountsshare.Command(accountsshare.Deps{Now: m.host.Clock.Now}, env),
		accountstakeback.Command(env),
	)
}

// invariant: station secrets and its verbs are the hidden v2 words of
// station accounts for one release (SPEC §4.3); allow and disallow are
// refused, the share list being the owner's to write.
func secretsAlias(env kit.Env, m machine) *cobra.Command {
	return kit.Alias("secrets", accountscmd.Command(accountscmd.Deps{Here: m.where}, env,
		kit.AliasArgs(kit.Alias("lend", accountsshare.Lend(accountsshare.Deps{Now: m.host.Clock.Now}, env), "share"), "<account>"),
		kit.AliasArgs(kit.Alias("recall", accountstakeback.Recall(env), "take-back"), "<account>"),
		accountscmd.Retired("allow", env),
		accountscmd.Retired("disallow", env),
	), "accounts")
}

func (m machine) secretStores() secrets.Stores {
	return secrets.Stores{
		ShareList: m.stores.shareList(), Hosts: m.stores.hosts(), Name: m.stores.name(),
		Shares: m.stores.shares, Machines: m.stores.machines,
	}
}

// value: whether a VM of this Mac runs, as its engine says, asked through
// the engine's socket.
func (m machine) vmRunning(ctx context.Context, vm string) (bool, error) {
	live, err := m.hostDaemon().VMs(ctx)
	if err != nil {
		return false, err
	}
	return live[vm].State == engine.StateRunning, nil
}
