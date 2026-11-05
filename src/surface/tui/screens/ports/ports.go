package ports

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: the sections of the Ports page by class (SPEC §5.6): what needs
// you, the declared public ports, the shared private ones.
const (
	needsYou  = "needs-you"
	public    = "public"
	private   = "private"
	localOnly = "local-only"
)

type Screen struct{}

func (Screen) Live() time.Duration { return 0 }

// value: a all unfolds the ports that listen on their machine only.
const all = "all"

func (Screen) Words(_ screen.Machine, words []string, st screen.State) []string {
	if st[all] == all {
		return append(slices.Clone(words), "--all")
	}
	return words
}

func (Screen) Title(v screen.View) string {
	n := 0
	for _, s := range v.Page.Sections {
		if slices.Contains([]string{needsYou, public, private}, s.ID) {
			n += len(s.Items)
		}
	}
	where := v.Machine.Label
	if v.Machine.Kind == "" {
		where = "all machines"
	}
	return "Ports" + ui.Sep + where + ui.Sep + strconv.Itoa(n)
}

func (Screen) Wide(screen.Machine) bool {
	// value: SPEC §9.2 draws Ports as one wide pane; enter opens a row's
	// detail, its QR code with it, over it.
	return true
}

func (Screen) Skips(_ screen.Machine, s page.Section) bool {
	// value: the cursor walks the ports, never the lines around them.
	return !slices.Contains([]string{needsYou, public, private, localOnly}, s.ID)
}

func (Screen) Lines(v screen.View) ([]string, bool) {
	t := ui.PortTable{Cursor: ui.Cursor{Key: v.Cursor, Focused: v.Focused, Dark: v.Dark, NoColor: v.NoColor, Width: v.Width, Room: v.Room}}
	for _, s := range v.Page.Sections {
		switch s.ID {
		case needsYou, public, private, localOnly:
			t.Classes = append(t.Classes, class(v, s))
		case "tailnet":
			for _, it := range s.Items {
				if r := it.Row; r != nil && r.Mark != page.OK && len(r.Cells) > 0 {
					t.Ending = append(t.Ending, ui.Check{Mark: r.Mark, Label: r.Label + " " + ui.Plain(r.Cells[0])})
				}
			}
		}
	}
	t.Ending = append(t.Ending, ui.Check{Label: "Station reads what listens, then compares it with what you declared."})
	for _, s := range v.Page.Sections {
		for _, it := range s.Items {
			if n := it.Note; n != nil && strings.HasSuffix(n.Text, "machine only") {
				t.Ending = append(t.Ending, ui.Check{Label: n.Text, Text: "a all"})
			}
		}
	}
	return t.Lines(), true
}

// value: a class's title in the screen's words: what it is, then how many.
func title(s page.Section, n int) string {
	head, rest, _ := strings.Cut(s.Title, ": ")
	words := []string{head}
	if rest != "" {
		if r, ok := strings.CutSuffix(rest, ", declared"); ok {
			words = append(words, r, "declared")
		} else {
			words = append(words, rest)
		}
	}
	return strings.Join(append(words, strconv.Itoa(n)), ui.Sep)
}

func class(v screen.View, s page.Section) ui.PortClass {
	c := ui.PortClass{Mark: page.Unjudged}
	exposed := []string{"MACHINE", "PORT", "WHAT", "LISTENS ON", "DECLARED"}
	switch s.ID {
	case needsYou:
		c.Mark, c.Heads, c.Or = page.Broken, exposed, ui.Pair{Label: "d declare public", Text: "declare a port you mean to open"}
	case public:
		c.Heads = []string{"MACHINE", "PORT", "WHAT", "ROUTE", "VERIFIED"}
	case localOnly:
		c.Heads = exposed
	}
	for _, it := range s.Items {
		if it.Empty != nil {
			c.Empty = "No " + it.Empty.Noun
		}
		r := it.Row
		if r == nil || (v.Match != nil && !v.Match(*r)) {
			continue
		}
		row := ui.PortRow{Key: s.ID + "/" + r.ID, Mark: r.Mark, Machine: cmp.Or(r.Short, r.Label), LastMark: page.Unjudged}
		row.Port, _, _ = strings.Cut(ui.Plain(screen.Cell(s, *r, "port")), " ")
		switch s.ID {
		case private:
			row.What = ui.Plain(screen.Cell(s, *r, "process"))
			row.Where = screen.Cell(s, *r, "url")
			if row.Where.IsNone() {
				row.Where = screen.Cell(s, *r, "state")
			}
		case public:
			row.What = ui.Plain(screen.Cell(s, *r, "what"))
			row.Where, row.Last, row.LastMark = route(screen.Cell(s, *r, "declared"), r.Mark)
		default:
			row.What = ui.Plain(screen.Cell(s, *r, "what"))
			row.Where = screen.Cell(s, *r, "listens")
			row.Last = strings.TrimPrefix(ui.Plain(screen.Cell(s, *r, "declared")), "declared ")
			if row.Last == "" {
				row.Last = "no"
			}
		}
		if h := r.Hint; s.ID == needsYou && h != nil && h.Command != "" && !slices.Contains(c.Fix, h.Command+" off") {
			c.Fix = append(c.Fix, h.Command+" off")
		}
		c.Rows = append(c.Rows, row)
	}
	c.Title = title(s, len(c.Rows))
	return c
}

// value: a declared port's route and what was verified of it, from its
// page's words: the route, then what proves it.
func route(v page.Value, m page.Mark) (page.Value, string, page.Mark) {
	words := make([]string, 0, len(v.Items))
	for _, w := range v.Items {
		words = append(words, ui.Plain(w))
	}
	switch {
	case len(words) == 0:
		return page.None(), strings.TrimPrefix(ui.Plain(v), "declared, but "), m
	case words[0] == "declared" && len(words) > 1:
		return page.Text(words[1]), "not verified yet", page.Unjudged
	}
	return page.Text(words[0]), strings.Join(words[1:], ui.Sep), m
}

func (Screen) Keys(v screen.View) []ui.Binding {
	var keys []ui.Binding
	switch v.Of.ID {
	case needsYou:
		keys = append(keys, screen.Bind("d", "declare public"))
	case public:
		keys = append(keys, screen.Bind("u", "undeclare"))
	case private:
		keys = append(keys, screen.Bind("x", "stop sharing"), screen.Bind("c", "copy the link"))
	}
	if sharer(v.Machine) {
		keys = append(keys, screen.Bind("s", "share"))
	}
	return append(keys, screen.Bind("a", "all"))
}

// value: a VM's Ports shares one of its ports; All machines asks which VM.
func sharer(m screen.Machine) bool {
	return m.Kind == "" || m.Kind == machine.Workspace || m.Kind == machine.Server
}

func port(v screen.View) (name, number string, ok bool) {
	if !v.HasRow {
		return "", "", false
	}
	name, number, ok = strings.Cut(v.Row.ID, ":")
	return name, number, ok
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	// invariant: SPEC §9.4: each key on the row it acts on; enter opens the
	// row's detail, with its QR code, as every list does.
	name, number, onPort := port(v)
	what := name + " " + number
	switch {
	case key == "c" && onPort && v.Of.ID == private:
		link := ui.Plain(screen.Cell(v.Of, v.Row, "url"))
		if link == "" {
			return screen.Effect{Say: what + " has no link to copy"}, true
		}
		return screen.Effect{Copy: link}, true
	case key == "d" && onPort && v.Of.ID == needsYou:
		return screen.Effect{Call: &screen.Call{Doing: "Declaring " + what + " public", Done: "Declared " + what + " public",
			Form: &screen.Form{Title: "Declare " + what + " public", Fields: []screen.Field{{Label: "Route", Value: "lan", Note: "the route that reaches it, as --via names it"}},
				Words: func(answers []string) []string {
					return []string{"ports", "declare", name, number, "--via", strings.TrimSpace(answers[0])}
				}}}}, true
	case key == "u" && onPort && v.Of.ID == public:
		return screen.Effect{Call: &screen.Call{Words: []string{"ports", "undeclare", name, number, "--yes"},
			Ask:   &screen.Ask{Question: "Undeclare " + what + "?", Lines: []string{"It needs you again while it is reachable from outside."}, Yes: "undeclare"},
			Doing: "Undeclaring " + what, Done: "Undeclared " + what}}, true
	case key == "x" && onPort && v.Of.ID == private:
		return screen.Effect{Call: &screen.Call{Words: []string{"ports", "stop-sharing", name, number, "--yes"},
			Ask:   &screen.Ask{Question: "Stop sharing " + what + "?", Lines: []string{"Its reserved port is released; it is not shared again until you share it."}, Yes: "stop sharing"},
			Doing: "Stopping the sharing of " + what, Done: "Stopped sharing " + what}}, true
	case key == "s" && sharer(v.Machine):
		return share(v.Machine), true
	case key == "a":
		return screen.Effect{State: v.State.With(all, map[bool]string{true: "", false: all}[v.State[all] == all]), Read: true}, true
	}
	return screen.Effect{}, false
}

func share(m screen.Machine) screen.Effect {
	fields := []screen.Field{{Label: "Port", Note: "the port it listens on inside"}}
	if m.Kind == "" {
		fields = append([]screen.Field{{Label: "VM", Note: "the VM whose port it is"}}, fields...)
	}
	title := "Share a port"
	if m.Kind != "" {
		title += " of " + m.Label
	}
	return screen.Effect{Call: &screen.Call{Doing: "Sharing a port", Done: "Shared it",
		Form: &screen.Form{Title: title, Fields: fields, Words: func(answers []string) []string {
			vm := m.Name
			if m.Kind == "" {
				vm, answers = strings.TrimSpace(answers[0]), answers[1:]
			}
			return []string{"ports", "share", vm, strings.TrimSpace(answers[0])}
		}}}}
}
