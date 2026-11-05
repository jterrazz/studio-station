package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	activityscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/activity"
	awakescreen "github.com/jterrazz/studio-station/src/surface/tui/screens/awake"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
	portsscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/ports"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	serverscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/server"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type Calls func(ctx context.Context, words []string) (string, error)

type called struct {
	t   task
	out string
	err error
}

type ticked struct {
	key string
	gen int
}

// value: a question or a form on the screen, and the call it leads to.
type dialog struct {
	call    screen.Call
	input   string
	answers []string
	at      int
}

func screenFor(n node, id section.ID) screen.Screen {
	vm := n.kind == machine.Workspace || n.kind == machine.Server
	own := map[section.ID]screen.Screen{section.Activity: activityscreen.Screen{}, section.Awake: awakescreen.Screen{}, section.Ports: portsscreen.Screen{}}
	if sc, ok := own[id]; ok {
		return sc
	}
	if vm && (id == section.Overview || id == section.SSH) {
		return serverscreen.Screen{}
	}
	return nil
}

func (s shell) screen() screen.Screen { return screenFor(s.current(), s.section()) }

func (s shell) machineOf(n node) screen.Machine {
	m := screen.Machine{Name: n.name, Label: n.label, Kind: n.kind, Mac: n.mac, Here: n.here}
	if mac, ok := find(s.nodes, n.mac); ok {
		m.MacLabel, m.MacHere = mac.label, mac.here
	}
	return m
}

// invariant: a screen reads the CLI's page for its words, and adds to them
// only what it keeps of its own (the window its graphs draw).
func (s shell) words(n node, id section.ID) []string {
	w := words(n, id)
	if sc := screenFor(n, id); sc != nil {
		return sc.Words(s.machineOf(n), w, s.states[s.key(n, id)])
	}
	return w
}

func (s shell) view() screen.View {
	key := s.key(s.current(), s.section())
	l := s.list(key)
	v := screen.View{Machine: s.machineOf(s.current()), Section: s.section(), Page: l.Page, Width: s.masterWidth() - 2,
		Room: max(s.height-7, 1), Dark: s.dark, NoColor: s.deps.NoColor, State: s.states[key], Beside: map[string]page.Page{}}
	v.Of, v.Row, v.HasRow = l.Current()
	v.Cursor, v.Focused, v.Match, v.Samples = l.Cursor, s.focus == ui.PaneMaster || s.focus == ui.PaneDetail, l.Matches, s.samples
	for _, r := range s.besides(s.current(), s.section()) {
		if got, ok := s.reads[besideKey(s.current(), r)]; ok && got.done && got.err == nil {
			v.Beside[r.Name] = got.page
		}
	}
	return v
}

func (s shell) besides(n node, id section.ID) []screen.Read {
	b, ok := screenFor(n, id).(screen.Beside)
	if !ok {
		return nil
	}
	return b.Reads(s.machineOf(n), id, func(other section.ID) []string { return words(n, other) })
}

func besideKey(n node, r screen.Read) string {
	return n.name + "\x00beside\x00" + strings.Join(r.Words, " ")
}

// value: the fixed panes the shown screen draws at the right of its
// master pane, once its page is read.
func (s shell) panes() []ui.Panel {
	if !s.paned() {
		return nil
	}
	b, _ := s.screen().(screen.Beside)
	return b.Panes(s.view())
}

func (s shell) paned() bool {
	b, ok := s.screen().(screen.Beside)
	return ok && s.read() && b.Paned(s.machineOf(s.current()), s.section())
}

func (s shell) effect(e screen.Effect) (shell, tea.Cmd) {
	key := s.key(s.current(), s.section())
	if e.State != nil {
		s.states = with(s.states, key, e.State)
	}
	var cmds []tea.Cmd
	if e.Say != "" {
		s = s.toast(e.Say)
		cmds = append(cmds, s.fade(s.acts.next))
	}
	if e.Copy != "" {
		s = s.toast("Copied " + e.Copy)
		cmds = append(cmds, tea.SetClipboard(e.Copy), s.fade(s.acts.next))
	}
	if e.Read {
		var cmd tea.Cmd
		s, cmd = s.again()
		cmds = append(cmds, cmd)
	}
	if e.Open != nil {
		m, cmd := s.open(e.Open.Machine)
		s, _ = m.(shell)
		for i, id := range s.sections(s.current()) {
			if id == e.Open.Section {
				m, cmd = s.show(i)
				s, _ = m.(shell)
			}
		}
		cmds = append(cmds, cmd)
	}
	if c := e.Call; c != nil {
		if c.Ask != nil || c.Form != nil {
			d := &dialog{call: *c}
			if c.Form != nil {
				for _, f := range c.Form.Fields {
					d.answers = append(d.answers, f.Value)
				}
			}
			s.dialog = d
			return s, tea.Batch(cmds...)
		}
		var cmd tea.Cmd
		s, cmd = s.call(*c)
		cmds = append(cmds, cmd)
	}
	return s, tea.Batch(cmds...)
}

// invariant: a screen's call is a task of the one Progress strip, as a
// gesture's command is: its spinner, then its toast or its failure.
func (s shell) call(c screen.Call) (shell, tea.Cmd) {
	s.acts.next++
	t := task{id: s.acts.next, step: gesture.Step{Words: c.Words, Doing: c.Doing, Done: c.Done}, copy: c.Copy, n: 1, total: 1}
	s.acts.tasks = append(slices.Clone(s.acts.tasks), t)
	ctx, run, w := s.ctx, s.deps.Run, c.Words
	return s, func() tea.Msg {
		if run == nil {
			return called{t: t, err: &Failed{Message: "this station runs no call"}}
		}
		out, err := run(ctx, w)
		return called{t: t, out: out, err: err}
	}
}

// invariant: a call that ended is said for a while, then the screen it
// changed and the machine's Overview and tree are read again.
func (s shell) finished(msg called) (shell, tea.Cmd) {
	t := msg.t
	var cmds []tea.Cmd
	switch {
	case msg.err != nil:
		t.state, t.err = failedTask, worded(msg.err)
	case t.copy:
		text := strings.TrimSpace(msg.out)
		t.state, t.step.Done = succeeded, "Copied "+text
		cmds = append(cmds, tea.SetClipboard(text), s.fade(t.id))
	default:
		t.state = succeeded
		cmds = append(cmds, s.fade(t.id))
	}
	s.acts.tasks = slices.Clone(s.acts.tasks)
	for i := range s.acts.tasks {
		if s.acts.tasks[i].id == t.id {
			s.acts.tasks[i] = t
		}
	}
	if t.copy && msg.err == nil {
		return s, tea.Batch(cmds...)
	}
	n := s.current()
	for _, key := range []string{s.key(n, s.section()), s.key(n, section.Overview), machinesKey} {
		s.asked = with(s.asked, key, false)
	}
	if n.mac != "" {
		if mac, ok := find(s.nodes, n.mac); ok {
			s.asked = with(s.asked, s.key(mac, section.Overview), false)
		}
	}
	m, cmd := s.load()
	cmds = append(cmds, cmd)
	m, cmd = m.ask(s.key(n, section.Overview), s.words(n, section.Overview))
	cmds = append(cmds, cmd)
	m, cmd = m.ask(machinesKey, []string{"machines"})
	return m, tea.Batch(append(cmds, cmd)...)
}

func (s shell) asking(k tea.KeyPressMsg) (shell, tea.Cmd) {
	d := *s.dialog
	c := d.call
	key := k.String()
	if key == "esc" {
		s.dialog = nil
		return s, nil
	}
	switch {
	case c.Form != nil:
		switch key {
		case "tab", "down":
			d.at = (d.at + 1) % len(d.answers)
		case "shift+tab", "up":
			d.at = (d.at + len(d.answers) - 1) % len(d.answers)
		case "backspace":
			if r := []rune(d.answers[d.at]); len(r) > 0 {
				d.answers = set(d.answers, d.at, string(r[:len(r)-1]))
			}
		case "enter":
			c.Words, c.Form = c.Form.Words(d.answers), nil
			s.dialog = nil
			return s.call(c)
		default:
			if t := k.Key().Text; t != "" {
				d.answers = set(d.answers, d.at, d.answers[d.at]+t)
			}
		}
	case c.Ask.Type != "":
		switch key {
		case "backspace":
			if r := []rune(d.input); len(r) > 0 {
				d.input = string(r[:len(r)-1])
			}
		case "enter":
			if d.input == c.Ask.Type {
				s.dialog = nil
				return s.call(c)
			}
		default:
			d.input += k.Key().Text
		}
	default:
		switch key {
		case "y", "enter":
			s.dialog = nil
			return s.call(c)
		case "n":
			s.dialog = nil
			return s, nil
		}
	}
	s.dialog = &d
	return s, nil
}

func set(xs []string, i int, x string) []string {
	out := append([]string(nil), xs...)
	out[i] = x
	return out
}

func (s shell) dialogPanel() *ui.Panel {
	if s.dialog == nil {
		return nil
	}
	d, c := s.dialog, s.dialog.call
	var p ui.Panel
	if f := c.Form; f != nil {
		form := ui.Form{Title: f.Title, Values: d.answers, At: d.at}
		for _, field := range f.Fields {
			form.Labels, form.Notes = append(form.Labels, field.Label), append(form.Notes, field.Note)
		}
		p = form.Panel()
	} else {
		p = ui.Ask{Question: c.Ask.Question, Lines: c.Ask.Lines, Yes: c.Ask.Yes, Type: c.Ask.Type, Input: d.input}.Panel()
	}
	return &p
}

// invariant: a live section is read again while it is shown, one chain of
// reads at a time: a newer read starts a newer chain.
func (s shell) live(key string) (shell, tea.Cmd) {
	sc := s.screen()
	if sc == nil || sc.Live() == 0 || key != s.key(s.current(), s.section()) {
		return s, nil
	}
	gen := s.gens[key] + 1
	s.gens = with(s.gens, key, gen)
	return s, tea.Tick(sc.Live(), func(time.Time) tea.Msg { return ticked{key: key, gen: gen} })
}

func (s shell) relive(msg ticked) (shell, tea.Cmd) {
	if msg.gen != s.gens[msg.key] || msg.key != s.key(s.current(), s.section()) || s.dialog != nil {
		return s, nil
	}
	return s.again()
}
