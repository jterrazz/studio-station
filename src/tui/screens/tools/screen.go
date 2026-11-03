package toolsscreen

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	pagetools "github.com/jterrazz/studio-station/src/pages/tools"
	"github.com/jterrazz/studio-station/src/tui/kit"
	"github.com/jterrazz/studio-station/src/ui/render/tui"
)

const ID = "tools"

type Tools interface {
	Watch(ctx context.Context) <-chan []tools.Report
	Forget()
	CheckInstall(ctx context.Context, names []string) error
	Install(ctx context.Context, r tools.InstallRequest) ([]tools.Outcome, error)
	CheckUninstall(names []string) error
	Uninstall(ctx context.Context, names []string) ([]tools.Outcome, error)
	CheckUpgrade(names []string) error
	Upgrade(ctx context.Context, r tools.UpgradeRequest) ([]tools.Outcome, error)
}

type Model struct {
	kit.Paged
	ctx      context.Context
	tools    Tools
	here     page.Context
	describe kit.Describe
	browser  tui.Browser
	loading  bool
	busy     bool
	gen      int
	// invariant: Init runs on a copy, so the running watch's cancel is held
	// where every copy of the screen sees it.
	stop *context.CancelFunc
}

type act struct{ key, name string }

type loaded struct {
	gen   int
	page  page.Page
	more  <-chan []tools.Report
	final bool
}

func New(ctx context.Context, t Tools, here page.Context, describe kit.Describe) Model {
	return Model{ctx: ctx, tools: t, here: here, describe: describe, browser: tui.New("Tools", "tools"), loading: true, stop: new(context.CancelFunc)}
}

func (m Model) Init() tea.Cmd { return m.watch(false) }

func (m Model) watch(fresh bool) tea.Cmd {
	if *m.stop != nil {
		(*m.stop)()
	}
	ctx, stop := context.WithCancel(context.Background()) //nolint:gosec // outside: held in *m.stop, called when the next watch starts.
	*m.stop = stop
	gen, t, here := m.gen, m.tools, m.here
	return func() tea.Msg {
		if fresh {
			t.Forget()
		}
		return next(gen, here, t.Watch(ctx))()
	}
}

func next(gen int, here page.Context, ch <-chan []tools.Report) tea.Cmd {
	return func() tea.Msg {
		reports, open := <-ch
		if !open {
			return loaded{gen: gen, final: true}
		}
		return loaded{gen: gen, page: pagetools.Catalogue(reports, here), more: ch}
	}
}

func (m Model) Focus() kit.Focus {
	switch m.browser.Focus() {
	case tui.Filter:
		return kit.FocusFilter
	case tui.Modal, tui.Sheet:
		return kit.FocusModal
	case tui.Normal, tui.Detail:
	}
	return kit.FocusNormal
}

func (m Model) Busy() bool { return m.busy }

func (m Model) Loading() bool { return m.loading }

func (m Model) View(width, height int) string { return m.browser.Body(width, height) }

func (m Model) Footer(width int) string { return m.browser.Footer(width, m.busy) }

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loaded:
		if msg.gen != m.gen {
			return m, nil
		}
		if msg.final {
			m.loading = false
			return m, nil
		}
		m.browser = m.browser.Show(msg.page)
		return m, next(msg.gen, m.here, msg.more)
	case kit.Done:
		if msg.Screen != ID {
			return m, nil
		}
		m.busy = false
		status := msg.Status
		if msg.Err != nil {
			status = kit.Failed(m.describe, msg.Err)
		}
		m.loading = true
		m.gen++
		return m, tea.Batch(kit.Say(status), m.watch(true))
	case kit.Reload:
		if m.busy {
			return m, nil
		}
		m.loading = true
		m.gen++
		return m, m.watch(true)
	case act:
		m.busy = true
		return m, m.run(msg)
	}
	var cmd tea.Cmd
	var ev *tui.Event
	m.browser, cmd, ev = m.browser.Update(msg)
	if ev == nil || ev.Row == "" || m.busy {
		return m, cmd
	}
	do := act{key: ev.Action.Key, name: ev.Row}
	if ev.Action.Confirm == nil {
		return m, func() tea.Msg { return do }
	}
	m.browser = m.browser.Ask(pagetools.Confirm(ev.Action.Confirm.Yes, do.name), func() tea.Msg { return do })
	return m, cmd
}

func (m Model) run(a act) tea.Cmd {
	return kit.Exec(ID, func() (kit.Status, error) {
		switch a.key {
		case pagetools.KeyInstall:
			outcomes, err := m.install(a.name)
			return outcome("installed", a.name, outcomes), err
		case pagetools.KeyUninstall:
			outcomes, err := m.uninstall(a.name)
			return outcome("uninstalled", a.name, outcomes), err
		}
		outcomes, err := m.upgrade(a.name)
		return outcome("upgraded", a.name, outcomes), err
	})
}

func (m Model) install(name string) ([]tools.Outcome, error) {
	if err := m.tools.CheckInstall(m.ctx, []string{name}); err != nil {
		return nil, err
	}
	return m.tools.Install(m.ctx, tools.InstallRequest{Names: []string{name}, Interactive: true})
}

func (m Model) uninstall(name string) ([]tools.Outcome, error) {
	if err := m.tools.CheckUninstall([]string{name}); err != nil {
		return nil, err
	}
	return m.tools.Uninstall(m.ctx, []string{name})
}

func (m Model) upgrade(name string) ([]tools.Outcome, error) {
	if err := m.tools.CheckUpgrade([]string{name}); err != nil {
		return nil, err
	}
	return m.tools.Upgrade(m.ctx, tools.UpgradeRequest{Names: []string{name}})
}

// value: the status bar names the tool asked for; a Then step that failed
// degrades the line rather than hiding behind the tool's own success.
func outcome(done, name string, outcomes []tools.Outcome) kit.Status {
	for _, o := range outcomes {
		if o.Name != name {
			continue
		}
		state := o.Result
		for _, s := range o.Steps {
			if s.Result == catalog.StateBroken && state == catalog.StateOK {
				state = catalog.StateDegraded
			}
		}
		return kit.Outcome(done, name, state, o.Detail)
	}
	return kit.Outcome(done, name, catalog.StateOK, "")
}
