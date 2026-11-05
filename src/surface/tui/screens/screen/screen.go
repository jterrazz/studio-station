package screen

import (
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type Machine struct {
	// value: the machine a screen shows, as the sidebar names it; a Kind
	// of "" is All machines.
	Name, Label string
	Kind        machine.Kind
	// value: the Mac a VM is on, by name and as the sidebar says it, and
	// whether it is this one.
	Mac, MacLabel string
	MacHere       bool
	Here          bool
}

type State map[string]string

func (s State) With(key, value string) State {
	// value: a screen's own memory (the window a graph draws, a sort), kept
	// per machine and section while the TUI runs.
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
	// value: the row under the cursor, in its section, and the list's key
	// of it; Focused while the cursor is drawn.
	Row     page.Row
	Of      page.Section
	HasRow  bool
	Cursor  string
	Focused bool
	// value: whether the list's filter keeps a row.
	Match func(page.Row) bool
	// value: each machine's last CPU percents the TUI has read, oldest
	// first, by its name.
	Samples map[string][]int
	Width   int
	Room    int
	Dark    bool
	NoColor bool
	State   State
	// value: the pages read beside the section's own, by their Read's name.
	Beside map[string]page.Page
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

type Call struct {
	// value: the CLI's own words of the call a key makes, after its question
	// or its form when it has one, said in the Progress strip while it runs.
	Words       []string
	Doing, Done string
	Ask         *Ask
	Form        *Form
	// value: what the call prints is copied once it ends (an ssh command).
	Copy bool
}

type Ask struct {
	// value: a yes/no question, or, with Type, the name to type exactly
	// before enter does it (SPEC §9.8).
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
	// a page read once.
	Live() time.Duration
}

func Cell(s page.Section, r page.Row, key string) page.Value {
	// value: a row's cell by its column's key, never by its place.
	for i, c := range s.Columns {
		if c.Key == key && i > 0 && i-1 < len(r.Cells) {
			return r.Cells[i-1]
		}
	}
	return page.None()
}

func Bind(key, label string) ui.Binding { return ui.Binding{Key: key, Label: label} }

type Wide interface {
	// value: a screen whose master pane takes the room of the detail too
	// (SPEC §9.2's one wide pane); enter opens the detail over it.
	Wide(m Machine) bool
}

type Skips interface {
	// value: a screen that draws its own pane keeps the list's cursor off
	// the sections it draws no row of.
	Skips(m Machine, s page.Section) bool
}

type Read struct {
	Name  string
	Words []string
}

type Beside interface {
	// value: a screen that draws more than its section's page: the pages
	// it reads beside it, and the fixed panes it draws at the right of the
	// master pane (a server's Overview: its Health and its Ports).
	// value: the reads beside the section's own, from the CLI's words of
	// each section of the machine.
	Reads(m Machine, id section.ID, words func(section.ID) []string) []Read
	// value: whether the screen draws panes at the right for this section.
	Paned(m Machine, id section.ID) bool
	Panes(v View) []ui.Panel
}
