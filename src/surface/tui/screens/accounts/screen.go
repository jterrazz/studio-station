package accountsscreen

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "accounts"

type Deps struct {
	Calls    call.Router
	Target   machine.Ref
	Forget   func()
	LendList func() []string
}

type Model struct {
	tab.View[accounts.Report]
	ctx context.Context
	d   Deps
}

func New(ctx context.Context, d Deps, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan accounts.Report, error) {
		if fresh {
			d.Forget()
		}
		return tab.Values(call.Follow(ctx, d.Calls, d.Target, accountscalls.Read, accountscalls.ReadIn{}, nil))
	}
	build := func(r accounts.Report) page.Page { return pageaccounts.Page(r, here) }
	return Model{View: tab.NewView(ID, "Accounts", "accounts", here, source, build, describe), ctx: ctx, d: d}
}

func (Model) OwnsReload() bool {
	// value: Accounts binds r to revoke (the owner's l and r), so its reload is R.
	return true
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil {
		return m, cmd
	}
	if ev.Action.Key == pageaccounts.KeyReload {
		view, reload, _ := m.View.Update(tab.Reload{})
		m.View = view
		return m, tea.Batch(cmd, reload)
	}
	mc, account := m.find(ev.Row)
	if mc == nil {
		return m, cmd
	}
	switch ev.Action.Key {
	case pageaccounts.KeyLend:
		return m.lend(*mc, account, cmd)
	case pageaccounts.KeyRevoke:
		return m.revoke(*mc, account, cmd)
	}
	return m, cmd
}

func (m Model) find(row string) (*accounts.Machine, *accounts.Account) {
	r := m.Last.Resolved()
	for i := range r.Machines {
		mc := &r.Machines[i]
		if row == mc.Name {
			return mc, nil
		}
		for j := range mc.Accounts {
			if accounts.RowID(mc.Name, mc.Accounts[j].Provider) == row {
				return mc, &mc.Accounts[j]
			}
		}
	}
	return nil, nil
}

func (m Model) lend(mc accounts.Machine, a *accounts.Account, cmd tea.Cmd) (tab.Screen, tea.Cmd) {
	if mc.Kind != accounts.KindVM {
		return m, tea.Batch(cmd, tab.Say(tab.Status{Mark: page.Absent, Text: "lend to a VM: select one of its rows"}))
	}
	list := accounts.Lendable(m.d.LendList(), "")
	provider := ""
	if a != nil {
		provider = a.Provider
	}
	checked := list
	if provider != "" {
		checked = accounts.Lendable(list, provider)
		if len(checked) == 0 {
			return m, tea.Batch(cmd, tab.Say(tab.Status{Mark: page.Absent, Text: "nothing on the lend list for " + provider}))
		}
	}
	if len(list) == 0 {
		return m, tea.Batch(cmd, tab.Say(tab.Status{Mark: page.Absent, Text: "nothing to lend: station secrets allow <name>"}))
	}
	vm := mc.Name
	var fill tea.Cmd
	m.View, fill = m.Fill(pageaccounts.LendForm(vm, list, checked), func(values map[string]string) tea.Cmd {
		names := strings.FieldsFunc(values["names"], func(r rune) bool { return r == ',' })
		term, err := time.ParseDuration(values["for"])
		if len(names) == 0 || err != nil {
			return tab.Say(tab.Status{Mark: page.Absent, Text: "nothing lent"})
		}
		return tab.Exec(ID, func() (tab.Status, error) {
			r, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, accountscalls.Lend, accountscalls.LendIn{To: vm, Names: names, Term: term}, nil)
			if err != nil {
				return tab.Status{}, err
			}
			return tab.Status{Mark: page.OK, Text: strings.Join(r.Names, ", ") + "  lent to " + vm + " until " + r.Until.Local().Format("15:04")}, nil
		})
	})
	return m, tea.Batch(cmd, fill)
}

func (m Model) revoke(mc accounts.Machine, a *accounts.Account, cmd tea.Cmd) (tab.Screen, tea.Cmd) {
	if a == nil {
		return m, cmd
	}
	if a.Lent == nil {
		text := "own account"
		for _, p := range catalog.Providers {
			if p.ID == a.Provider && p.Logout != "" {
				text = "own account: " + p.Logout
			}
		}
		return m, tea.Batch(cmd, tab.Say(tab.Status{Mark: page.Absent, Text: text}))
	}
	vm, name := mc.Name, a.Lent.Name
	run := tab.Exec(ID, func() (tab.Status, error) {
		_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, accountscalls.Recall, accountscalls.RecallIn{From: vm, Names: []string{name}}, nil)
		return tab.Outcome("recalled", name, catalog.StateOK, "from "+vm), err
	})
	var act tea.Cmd
	m.View, act = m.Act(run)
	return m, tea.Batch(cmd, act)
}
