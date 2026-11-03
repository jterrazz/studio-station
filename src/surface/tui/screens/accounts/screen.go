package accountsscreen

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/page"
	pageaccounts "github.com/jterrazz/studio-station/src/pages/accounts"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "accounts"

type Accounts interface {
	Read(ctx context.Context) <-chan accounts.Report
	LendList() []string
	Forget()
}

type Secrets interface {
	Lend(ctx context.Context, to string, names []string, term time.Duration) (secrets.LendReport, error)
	Recall(ctx context.Context, from string, names []string) (secrets.RecallReport, error)
}

type Model struct {
	tab.View[accounts.Report]
	ctx      context.Context
	accounts Accounts
	secrets  Secrets
}

func New(ctx context.Context, a Accounts, s Secrets, here page.Context, describe tab.Describe) Model {
	source := func(ctx context.Context, fresh bool) (<-chan accounts.Report, error) {
		if fresh {
			a.Forget()
		}
		return a.Read(ctx), nil
	}
	build := func(r accounts.Report) page.Page { return pageaccounts.Page(r, here) }
	return Model{View: tab.NewView(ID, "Accounts", "accounts", here, source, build, describe), ctx: ctx, accounts: a, secrets: s}
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
	machine, account := m.find(ev.Row)
	if machine == nil {
		return m, cmd
	}
	switch ev.Action.Key {
	case pageaccounts.KeyLend:
		return m.lend(*machine, account, cmd)
	case pageaccounts.KeyRevoke:
		return m.revoke(*machine, account, cmd)
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
	list := accounts.Lendable(m.accounts.LendList(), "")
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
			r, err := m.secrets.Lend(m.ctx, vm, names, term)
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
		_, err := m.secrets.Recall(m.ctx, vm, []string{name})
		return tab.Outcome("recalled", name, catalog.StateOK, "from "+vm), err
	})
	c := pageaccounts.RecallConfirm(vm, name)
	var act tea.Cmd
	m.View, act = m.Act(&c, run)
	return m, tea.Batch(cmd, act)
}
