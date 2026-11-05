package screens

import (
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: the machine a screen shows, as the sidebar names it; a Kind of ""
// is All machines.
type Machine struct {
	Name, Label string
	Kind        machine.Kind
	Mac         string
	Here        bool
}

// value: a screen's own memory (the window a graph draws, a sort), kept
// per machine and section while the TUI runs.
type State map[string]string

func (s State) With(key, value string) State {
	out := make(State, len(s)+1)
	for k, v := range s {
		out[k] = v
	}
	out[key] = value
	return out
}

type View struct {
	Machine Machine
	Section section.ID
	Page    page.Page
	// value: the row under the cursor, in its section.
	Row     page.Row
	Of      page.Section
	HasRow  bool
	Width   int
	Room    int
	Dark    bool
	NoColor bool
	State   State
}

type Effect struct {
	// value: the screen's state from now on; nil keeps it.
	State State
	Read  bool
	Call  *Call
	Copy  string
	Open  *Open
	// value: what the Progress strip says at once, with no call.
	Say string
}

type Open struct {
	Machine string
	Section section.ID
}

// value: a call the TUI makes by the CLI's own words, after its question or
// its form when it has one, said in the Progress strip while it runs.
type Call struct {
	Words       []string
	Doing, Done string
	Ask         *Ask
	Form        *Form
	// value: what the call prints is copied once it ends (an ssh command).
	Copy bool
}

// value: a yes/no question, or, with Type, the name to type exactly before
// enter does it (SPEC §9.8).
type Ask struct {
	Question string
	Lines    []string
	Type     string
	Yes      string
}

type Form struct {
	Title  string
	Fields []Field
	// value: the call's words from the answers, one per field in order.
	Words func(answers []string) []string
}

type Field struct {
	Label, Value, Note string
}

type Screen interface {
	// value: the words the section's page is read with, from the CLI's.
	Words(m Machine, words []string, st State) []string
	Title(v View) string
	// value: the master pane's lines; false lets the section's list draw.
	Lines(v View) ([]string, bool)
	Keys(v View) []ui.Binding
	Press(key string, v View) (Effect, bool)
	// value: how often the page is read again while it is shown; zero for
	// never (a watch, SPEC §9.10).
	Live() time.Duration
}

// value: how a machine is named after --on: none for This Mac.
func On(m Machine) []string {
	if m.Here || m.Name == "" {
		return nil
	}
	return []string{"--on", m.Name}
}

// value: a row's cell by its column's key, never by its place.
func Cell(s page.Section, r page.Row, key string) page.Value {
	for i, c := range s.Columns {
		if c.Key == key && i > 0 && i-1 < len(r.Cells) {
			return r.Cells[i-1]
		}
	}
	return page.None()
}

func Bind(key, label string) ui.Binding { return ui.Binding{Key: key, Label: label} }
