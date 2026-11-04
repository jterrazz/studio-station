package wire

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/accounts/adapter/cli"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/contract"
	accountscmd "github.com/jterrazz/studio-station/src/surface/cli/accounts"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) accounts() *accounts.Service {
	o := accounts.Options{Here: m.kind, HereName: m.where.Machine}
	ids := m.identities(engine.ChildrenDir(m.paths.State))
	if m.kind != catalog.Host {
		o.Keys = m.stationHostKeys
		return accounts.New(m.host, m.pool, ids, accounts.Ports{}, o)
	}
	sec := m.secrets()
	tn := m.tailnet()
	d := m.engineAPI()
	sources := accounts.Ports{
		LendList: sec.LendList,
		Tailnet: func(ctx context.Context) accounts.TailnetAccount {
			return tailnetAccount(ctx, tn)
		},
		VMs: func(ctx context.Context) ([]accounts.VM, error) {
			recs, err := m.machines(false).Records()
			if err != nil {
				return nil, err
			}
			lends, _ := m.stores.shares.List(ctx)
			live, _ := d.VMs(ctx)
			out := make([]accounts.VM, 0, len(recs))
			for _, r := range recs {
				vm := accounts.VM{Name: r.Name, Lifetime: string(r.Lifetime)}
				for _, v := range live {
					if v.Name == r.Name && v.State == engine.StateRunning {
						vm.Running = true
					}
				}
				for _, l := range lends[r.Name] {
					vm.Lends = append(vm.Lends, accounts.Lend(l))
				}
				out = append(out, vm)
			}
			return out, nil
		},
		Daemon: func(ctx context.Context) ([]accounts.VMRead, string) {
			var view engine.AccountsView
			if err := d.do(ctx, process.Accounts, nil, &view); err != nil {
				if _, missing := m.host.FS.Stat(catalog.EnginePlist(m.host.Env.Home())); missing != nil {
					return nil, accounts.DaemonAbsent
				}
				return nil, accounts.DaemonStopped
			}
			out := make([]accounts.VMRead, 0, len(view.VMs))
			for _, v := range view.VMs {
				read := accounts.VMRead{Name: v.Name, Guest: v.Guest}
				var rows []accounts.Account
				if json.Unmarshal(v.Accounts, &rows) == nil {
					for _, a := range rows {
						read.Accounts = append(read.Accounts, accounts.Redact(a))
					}
				}
				out = append(out, read)
			}
			return out, accounts.DaemonRunning
		},
	}
	return accounts.New(m.host, m.pool, ids, sources, o)
}

// value: inside a VM, who may ssh in is the station hosts block the guest
// writes into authorized_keys.
func (m machine) stationHostKeys() []string {
	data, err := m.host.FS.ReadFile(path.Join(m.host.Env.Home(), ".ssh", "authorized_keys"))
	if err != nil {
		return nil
	}
	var keys []string
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case guest.KeysBegin:
			inside = true
		case guest.KeysEnd:
			inside = false
		default:
			if inside && strings.TrimSpace(line) != "" {
				keys = append(keys, strings.TrimSpace(line))
			}
		}
	}
	return keys
}

func accountsCommand(env kit.Env, m machine) *cobra.Command {
	return accountscmd.Command(accountscmd.Deps{Here: m.where}, env)
}

func tailnetAccount(ctx context.Context, tn contract.Tailnet) accounts.TailnetAccount {
	self, err := tn.Self(ctx)
	var absent *ports.Absent
	switch {
	case errors.As(err, &absent):
		return accounts.TailnetAccount{State: catalog.StateAbsent, Variant: self.Client.Variant}
	case err != nil:
		return accounts.TailnetAccount{State: catalog.StateBroken, Variant: self.Client.Variant}
	}
	return accounts.TailnetAccount{State: catalog.StateOK, Login: self.Login, Tailnet: self.Network, Variant: self.Client.Variant, Problem: self.Sharing.Backend}
}

// value: who each provider says is signed in here, read by its own tool, each
// read's process recorded under track.
func (m machine) identities(track string) accountscontract.Identities {
	if m.world != nil {
		return m.world.identities()
	}
	return cli.New(m.host.Run, m.host.FS, cli.Options{Home: m.host.Env.Home(), Track: track})
}
