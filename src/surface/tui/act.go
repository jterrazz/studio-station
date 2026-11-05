package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/tui/screens"
	activityscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/activity"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
	"github.com/jterrazz/studio-station/src/ui/token"
)

// value: runs station with the CLI's words, the call a key makes, and
// answers what it printed.
type Calls func(ctx context.Context, words []string) (string, error)

type called struct {
	id  int
	out string
	err error
}

type cleared struct{ id int }

type ticked struct {
	key string
	gen int
}

type job struct {
	id          int
	doing, done string
	copy        bool
}

// value: a question or a form on the screen, and the call it leads to.
type dialog struct {
	call    screens.Call
	input   string
	answers []string
	at      int
}

const toastFor = 4 * time.Second

func screenFor(n node, id section.ID) screens.Screen {
	switch id {
	case section.Activity:
		return activityscreen.Screen{}
	}
	return nil
}

func (s shell) screen() screens.Screen { return screenFor(s.current(), s.section()) }

func (s shell) machineOf(n node) screens.Machine {
	return screens.Machine{Name: n.name, Label: n.label, Kind: n.kind, Mac: n.mac, Here: n.here}
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

func (s shell) view() screens.View {
	key := s.key(s.current(), s.section())
	l := s.list(key)
	v := screens.View{Machine: s.machineOf(s.current()), Section: s.section(), Page: l.Page, Width: s.masterWidth() - 2,
		Room: max(s.height-7, 1), Dark: s.dark, NoColor: s.deps.NoColor, State: s.states[key]}
	v.Of, v.Row, v.HasRow = l.Current()
	return v
}

func (s shell) effect(e screens.Effect) (shell, tea.Cmd) {
	key := s.key(s.current(), s.section())
	if e.State != nil {
		s.states = with(s.states, key, e.State)
	}
	var cmds []tea.Cmd
	if e.Say != "" {
		var cmd tea.Cmd
		s, cmd = s.say(e.Say)
		cmds = append(cmds, cmd)
	}
	if e.Copy != "" {
		var cmd tea.Cmd
		s, cmd = s.say(ui.Glyph(page.OK) + " Copied " + e.Copy)
		cmds = append(cmds, tea.SetClipboard(e.Copy), cmd)
	}
	if e.Read {
		var cmd tea.Cmd
		s, cmd = s.again()
		cmds = append(cmds, cmd)
	}
	if e.Open != nil {
		m, cmd := s.open(e.Open.Machine)
		s = m.(shell)
		for i, id := range s.sections(s.current()) {
			if id == e.Open.Section {
				m, cmd = s.show(i)
				s = m.(shell)
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
		s, cmd = s.start(*c)
		cmds = append(cmds, cmd)
	}
	return s, tea.Batch(cmds...)
}

func (s shell) start(c screens.Call) (shell, tea.Cmd) {
	s.seq++
	j := job{id: s.seq, doing: c.Doing, done: c.Done, copy: c.Copy}
	s.jobs = append(append([]job(nil), s.jobs...), j)
	ctx, run, w := s.ctx, s.deps.Run, c.Words
	return s, func() tea.Msg {
		if run == nil {
			return called{id: j.id, err: &Failed{Message: "this station runs no call"}}
		}
		out, err := run(ctx, w)
		return called{id: j.id, out: out, err: err}
	}
}

// invariant: a call that ended is said for a while, then the screen it
// changed and the machine's Overview and tree are read again.
func (s shell) finished(msg called) (shell, tea.Cmd) {
	var j job
	kept := make([]job, 0, len(s.jobs))
	for _, x := range s.jobs {
		if x.id == msg.id {
			j = x
			continue
		}
		kept = append(kept, x)
	}
	s.jobs = kept
	var cmds []tea.Cmd
	var said string
	switch {
	case msg.err != nil:
		said = ui.Glyph(page.Broken) + " " + strings.TrimSuffix(msg.err.Error(), ".")
	case j.copy:
		text := strings.TrimSpace(msg.out)
		said = ui.Glyph(page.OK) + " Copied " + text
		cmds = append(cmds, tea.SetClipboard(text))
	default:
		said = ui.Glyph(page.OK) + " " + j.done
	}
	s, cmd := s.say(said)
	cmds = append(cmds, cmd)
	if j.copy && msg.err == nil {
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

func (s shell) say(text string) (shell, tea.Cmd) {
	s.seq++
	s.said, s.saidID = text, s.seq
	id := s.seq
	return s, tea.Tick(toastFor, func(time.Time) tea.Msg { return cleared{id: id} })
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
			return s.start(c)
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
				return s.start(c)
			}
		default:
			d.input += k.Key().Text
		}
	default:
		switch key {
		case "y", "enter":
			s.dialog = nil
			return s.start(c)
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
	if f := c.Form; f != nil {
		lines := []string{""}
		for i, field := range f.Fields {
			lead := "  "
			if i == d.at {
				lead = token.Cursor + " "
			}
			line := " " + lead + pad(field.Label, 12) + d.answers[i]
			if i == d.at {
				line += "_"
			}
			if field.Note != "" {
				line += "   " + field.Note
			}
			lines = append(lines, line)
		}
		lines = append(lines, "", " enter "+strings.ToLower(f.Title)+"   tab next field   esc cancel")
		return &ui.Panel{Title: f.Title, Lines: lines}
	}
	a := c.Ask
	lines := []string{"", " " + a.Question}
	for _, l := range a.Lines {
		lines = append(lines, " "+l)
	}
	yes := a.Yes
	if yes == "" {
		yes = "yes"
	}
	if a.Type != "" {
		lines = append(lines, "", " Type "+a.Type+" to confirm: "+d.input+"_")
		if d.input == a.Type {
			lines = append(lines, "", " enter "+yes+"   esc keep it")
		} else {
			lines = append(lines, "", " enter is off until you type "+a.Type+"   esc keep it")
		}
		return &ui.Panel{Title: a.Question, Lines: lines}
	}
	lines = append(lines, "", " y "+yes+"   n no")
	return &ui.Panel{Title: a.Question, Lines: lines}
}

func pad(s string, n int) string {
	if len([]rune(s)) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

func (s shell) progress() []string {
	var out []string
	parts := make([]string, 0, len(s.jobs)+1)
	for _, j := range s.jobs {
		parts = append(parts, string([]rune(token.Spinner)[0])+" "+j.doing)
	}
	if s.said != "" {
		parts = append(parts, s.said)
	}
	if len(parts) > 0 {
		out = append(out, "Progress  "+strings.Join(parts, "   "))
	}
	return out
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

func (s shell) tick(msg ticked) (shell, tea.Cmd) {
	if msg.gen != s.gens[msg.key] || msg.key != s.key(s.current(), s.section()) || s.dialog != nil {
		return s, nil
	}
	return s.again()
}
