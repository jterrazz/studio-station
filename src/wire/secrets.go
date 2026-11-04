package wire

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/remote/adapter/unix"
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

func receives(guest secrets.Guest) func(context.Context, string) (bool, error) {
	// value: a VM whose station is older than its kind's answer receives.
	ask := unix.New(sharedGuest{guest}, nil)
	return func(ctx context.Context, vm string) (bool, error) {
		kind, err := ask.KindOf(ctx, vm)
		if err != nil || kind == "" {
			return err == nil, err
		}
		return accounts.SharedInto(coremachine.Profile{Ref: coremachine.Ref{Name: vm}, Kind: kind}).OK, nil
	}
}

type sharedGuest struct{ g secrets.Guest }

func (s sharedGuest) GuestExec(ctx context.Context, vm, line, stdin string) (stdout string, code int, err error) {
	out, err := s.g.GuestExec(ctx, vm, line, stdin)
	return out.Stdout, out.ExitCode, err
}
