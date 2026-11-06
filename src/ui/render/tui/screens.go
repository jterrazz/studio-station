package tui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const (
	Ellipsis  = token.Ellipsis
	Arrow     = token.Arrow
	AllMachinesMark = token.AllMachines
	label     = 12
	graphMax  = 30
	graphMin  = 4
)

func Keyed(text, key string, width int) string {
	// value: a line of a list a modal draws: its text cut to the room, then
	// its key in a column of its own (the palette's entries).
	k := ""
	if key != "" {
		k = atom.KeyName(key)
	}
	return atom.Pad(atom.End(text, width-12), width-9) + k
}

type Activity struct {
	// value: an Activity screen's parts, as its page has them, to draw.
	Banner    []string
	Graphs    page.Section
	Values    page.Section
	Processes *page.Section
	// value: "memory" sorts the processes by memory, else by CPU.
	Sort string
	// value: how the engine samples what the screen shows, said in place of
	// the banner when the screen's own engine samples it.
	Sampled string
	Width   int
	Dark    bool
}

func (a Activity) Lines() []string {
	st := token.New(a.Dark)
	said := slices.DeleteFunc(slices.Clone(a.Banner), func(w string) bool { return strings.HasPrefix(w, "from ") })
	if a.Sampled != "" {
		said = []string{a.Sampled}
	}
	out := []string{" " + st.Muted.Render(strings.Join(said, token.Sep)), ""}
	room := a.Width - 1 - label - 2
	widest, drawn := 0, false
	for _, it := range a.Graphs.Items {
		if it.Row != nil && len(it.Row.Cells) > 1 {
			widest, drawn = max(widest, atom.Width(atom.Plain(it.Row.Cells[0]))), true
		}
	}
	width := min(graphMax, max(graphMin, room-widest))
	if !drawn {
		width = -2
	}
	for _, it := range a.Graphs.Items {
		if it.Row != nil {
			out = append(out, measure(*it.Row, width, room))
		}
	}
	for _, it := range a.Values.Items {
		if r := it.Row; r != nil && (r.ID == "thermal" || r.ID == "battery") {
			text := atom.Plain(r.Cells[0])
			if r.Mark != page.Unjudged {
				text = atom.Mark(st, r.Mark) + text
			}
			out = append(out, " "+atom.Pad(r.Label, label)+atom.End(text, max(a.Width-1-label, 1)))
		}
	}
	for _, w := range a.Banner {
		if up, ok := strings.CutPrefix(w, "up "); ok {
			out = append(out, " "+atom.Pad("Up", label)+up)
		}
	}
	if a.Processes != nil {
		out = append(append(out, ""), processes(st, *a.Processes, a.Sort, a.Width)...)
	}
	return out
}

func measure(r page.Row, width, room int) string {
	// invariant: SPEC §6.4, AC-24: a narrow pane shortens every graph alike,
	// never drops a measure; its words are cut only once the graph is at its
	// least. A measure with no points leaves its graph's place blank.
	text := atom.Plain(r.Cells[0])
	head := " " + atom.Pad(r.Label, label)
	if len(r.Cells) < 2 {
		return head + atom.Spaces(width+2) + atom.End(text, max(room-width, 1))
	}
	var graph string
	switch g := r.Cells[1]; g.Kind {
	case page.KindRatio:
		graph = atom.Bar(g.N, g.Of, width)
	default:
		samples := make([]int, 0, len(g.Items))
		for _, x := range g.Items[min(1, len(g.Items)):] {
			samples = append(samples, int(x.N))
		}
		graph = atom.Graph(samples, width)
	}
	return head + atom.Pad(graph, width) + "  " + atom.End(text, max(room-width, 1))
}

func cell(s page.Section, r page.Row, key string) page.Value {
	for i, c := range s.Columns {
		if c.Key == key && i > 0 && i-1 < len(r.Cells) {
			return r.Cells[i-1]
		}
	}
	return page.None()
}

func processes(st token.Styles, s page.Section, sort string, width int) []string {
	by, other, key := "CPU", "memory", "cpu"
	if sort == "memory" {
		by, other, key = "memory", "CPU", "memory"
	}
	title := " " + st.Strong.Render("TOP PROCESSES") + st.Muted.Render(token.Sep+"by "+by)
	hint := "p by " + other
	title = atom.Pad(title, max(width-atom.Width(hint)-2, atom.Width(title)+1)) + st.Muted.Render(hint)
	rows := make([]page.Row, 0, len(s.Items))
	for _, it := range s.Items {
		if it.Row != nil {
			rows = append(rows, *it.Row)
		}
	}
	slices.SortStableFunc(rows, func(a, b page.Row) int { return cmp.Compare(cell(s, b, key).N, cell(s, a, key).N) })
	out := make([]string, 0, 1+len(rows))
	out = append(out, title)
	for _, r := range rows {
		line := "   " + atom.Pad(atom.End(r.Label, 16), 16) + atom.Pad(atom.Plain(cell(s, r, "what")), 14) +
			atom.Pad(dash(cell(s, r, "cpu")), 8) + dash(cell(s, r, "memory"))
		out = append(out, atom.End(line, width))
	}
	return out
}

func dash(v page.Value) string {
	if t := atom.Show(v); t != "" {
		return t
	}
	return token.MarkNotFor
}

type Ask struct {
	// value: a question on the screen (SPEC §9.8): yes or no, or, with Type,
	// the name to type exactly before enter does it.
	Question, Yes, Type, Input string
	Lines                      []string
}

func (a Ask) Panel() Panel {
	lines := []string{"", " " + a.Question}
	for _, l := range a.Lines {
		lines = append(lines, " "+l)
	}
	yes := cmp.Or(a.Yes, "yes")
	switch {
	case a.Type == "":
		lines = append(lines, "", " y "+yes+"   n no")
	case a.Input == a.Type:
		lines = append(lines, "", " Type "+a.Type+" to "+yes+" it: "+a.Input+"_", "", " enter "+yes+"   esc keep it")
	default:
		lines = append(lines, "", " Type "+a.Type+" to "+yes+" it: "+a.Input+"_", "", " enter is off until you type "+a.Type+"   esc keep it")
	}
	return Panel{Title: a.Question, Lines: lines}
}

type Form struct {
	// value: a form on the screen, its fields' labels, answers and notes, the
	// cursor on one; a secret's answer is drawn masked, a choice's with the
	// values it turns through.
	Title                 string
	Labels, Values, Notes []string
	Secret                []bool
	Options               [][]string
	At                    int
}

func (f Form) Panel() Panel {
	lines := make([]string, 0, len(f.Labels)+3)
	lines = append(lines, "")
	for i, l := range f.Labels {
		lead := "  "
		if i == f.At {
			lead = token.Cursor + " "
		}
		value := f.Values[i]
		if i < len(f.Secret) && f.Secret[i] {
			value = strings.Repeat("•", len([]rune(value)))
		}
		choice := i < len(f.Options) && len(f.Options[i]) > 0
		switch {
		case choice:
			value = token.KeyLeft + " " + value + " " + token.KeyRight
		case i == f.At:
			value += "_"
		}
		line := " " + lead + atom.Pad(l, max(14, len([]rune(l))+2)) + value
		note := ""
		if i < len(f.Notes) {
			note = f.Notes[i]
		}
		if choice && note == "" {
			note = strings.Join(f.Options[i], token.Sep)
		}
		if note != "" {
			line += "   " + note
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", " enter "+strings.ToLower(f.Title)+"   tab next field   esc cancel")
	return Panel{Title: f.Title, Lines: lines}
}
