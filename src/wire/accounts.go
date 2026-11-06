package wire

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/adapter/cli"
	"github.com/jterrazz/studio-station/src/accounts/adapter/tailscale"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/guest"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/process"
)

func (m machine) accounts() *accountscalls.Reader {
	o := accountscalls.ReaderOptions{Here: m.kind, HereName: m.where.Machine, Now: m.host.Clock.Now}
	ids := m.identities(engine.ChildrenDir(m.paths.State))
	if m.kind != catalog.Host {
		o.Keys = m.stationHostKeys
		return accountscalls.NewReader(m.pool, ids, accountscalls.ReaderPorts{}, o)
	}
	sec := m.secrets()
	d := m.engineAPI()
	sources := accountscalls.ReaderPorts{
		LendList: sec.ShareList,
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
	return accountscalls.NewReader(m.pool, tailscale.New(m.tailnet(), ids), sources, o)
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

// value: who each provider says is signed in here, read by its own tool, each
// read's process recorded under track.
func (m machine) identities(track string) accountscontract.Identities {
	if m.world != nil {
		return m.world.identities()
	}
	return cli.New(m.host.Run, m.host.FS, cli.Options{Home: m.host.Env.Home(), Track: track})
}
