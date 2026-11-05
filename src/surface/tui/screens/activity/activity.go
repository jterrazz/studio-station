package activity

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
	"github.com/jterrazz/studio-station/src/surface/tui/screens"
	"github.com/jterrazz/studio-station/src/ui/atom"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type window struct {
	flag, words string
}

// value: SPEC §6.4's picker, `w` cycling it: 2 s points, then minutes.
var windows = []window{{"6m", "6 min"}, {"1h", "1 h"}, {"24h", "24 h"}}

const (
	label    = 12
	graphMax = 30
	graphMin = 4
)

type Screen struct{}

func (Screen) Live() time.Duration { return 2 * time.Second }

func of(st screens.State) window {
	for _, w := range windows {
		if w.flag == st["window"] {
			return w
		}
	}
	return windows[0]
}

func (Screen) Words(m screens.Machine, words []string, st screens.State) []string {
	if m.Kind == "" {
		return words
	}
	return append(slices.Clone(words), "--graph", of(st).flag)
}

func banner(p page.Page) []string {
	for _, s := range p.Sections {
		if s.Role == page.Banner && s.ID != page.OnSection {
			for _, it := range s.Items {
				if it.Row != nil && len(it.Row.Cells) > 0 {
					var words []string
					for _, w := range it.Row.Cells[0].Items {
						words = append(words, atom.Plain(w))
					}
					if len(words) == 0 {
						words = append(words, atom.Plain(it.Row.Cells[0]))
					}
					return words
				}
			}
		}
	}
	return nil
}

func (Screen) Title(v screens.View) string {
	if v.Machine.Kind == "" {
		n := 0
		for _, s := range v.Page.Sections {
			if s.ID == "machines" {
				n = len(s.Items)
			}
		}
		return "Activity" + token.Sep + "all machines" + token.Sep + strconv.Itoa(n)
	}
	title := "Activity" + token.Sep + v.Machine.Label
	for _, w := range banner(v.Page) {
		if strings.HasPrefix(w, "from ") {
			return title + token.Sep + w
		}
	}
	return title + token.Sep + "last " + of(v.State).words
}

func (Screen) Keys(v screens.View) []ui.Binding {
	if v.Machine.Kind == "" {
		return []ui.Binding{screens.Bind("enter", "open its Activity")}
	}
	return []ui.Binding{screens.Bind("w", "window"), screens.Bind("p", "sort processes")}
}

func (Screen) Press(key string, v screens.View) (screens.Effect, bool) {
	switch {
	case v.Machine.Kind == "" && key == "enter" && v.HasRow && v.Of.ID == "machines":
		return screens.Effect{Open: &screens.Open{Machine: v.Row.ID, Section: section.Activity}}, true
	case v.Machine.Kind == "":
		return screens.Effect{}, false
	case key == "w":
		i := slices.Index(windows, of(v.State))
		return screens.Effect{State: v.State.With("window", windows[(i+1)%len(windows)].flag), Read: true}, true
	case key == "p":
		sort := "memory"
		if v.State["sort"] == "memory" {
			sort = "cpu"
		}
		return screens.Effect{State: v.State.With("sort", sort)}, true
	}
	return screens.Effect{}, false
}

func find(p page.Page, id string) (page.Section, bool) {
	for _, s := range p.Sections {
		if s.ID == id {
			return s, true
		}
	}
	return page.Section{}, false
}

func (Screen) Lines(v screens.View) ([]string, bool) {
	graphs, ok := find(v.Page, pageactivity.Graphs)
	if v.Machine.Kind == "" || !ok {
		return nil, false
	}
	st := token.New(v.Dark)
	words := banner(v.Page)
	said := slices.DeleteFunc(slices.Clone(words), func(w string) bool { return strings.HasPrefix(w, "from ") })
	out := []string{" " + st.Muted.Render(strings.Join(said, token.Sep)), ""}
	room := v.Width - 1 - label - 2
	widest, drawn := 0, false
	for _, it := range graphs.Items {
		if it.Row != nil && len(it.Row.Cells) > 1 {
			widest, drawn = max(widest, atom.Width(atom.Plain(it.Row.Cells[0]))), true
		}
	}
	width := min(graphMax, max(graphMin, room-widest))
	if !drawn {
		width = -2
	}
	for _, it := range graphs.Items {
		if it.Row != nil {
			out = append(out, measure(*it.Row, width, room))
		}
	}
	if values, ok := find(v.Page, "values"); ok {
		for _, it := range values.Items {
			if r := it.Row; r != nil && (r.ID == "thermal" || r.ID == "battery") {
				text := atom.Plain(r.Cells[0])
				if r.Mark != page.Unjudged {
					text = atom.Mark(st, r.Mark) + text
				}
				out = append(out, " "+atom.Pad(r.Label, label)+atom.End(text, max(v.Width-1-label, 1)))
			}
		}
	}
	for _, w := range words {
		if up, ok := strings.CutPrefix(w, "up "); ok {
			out = append(out, " "+atom.Pad("Up", label)+up)
		}
	}
	if procs, ok := find(v.Page, pageactivity.Processes); ok {
		out = append(append(out, ""), processes(st, procs, v.State["sort"], v.Width)...)
	}
	return out, true
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
	return head + atom.PadLeft(graph, width) + "  " + atom.End(text, max(room-width, 1))
}

func processes(st token.Styles, s page.Section, sort string, width int) []string {
	by, other := "CPU", "memory"
	key := "cpu"
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
	slices.SortStableFunc(rows, func(a, b page.Row) int {
		return cmp.Compare(screens.Cell(s, b, key).N, screens.Cell(s, a, key).N)
	})
	out := make([]string, 0, 1+len(rows))
	out = append(out, title)
	for _, r := range rows {
		line := "   " + atom.Pad(atom.End(r.Label, 16), 16) + atom.Pad(atom.Plain(screens.Cell(s, r, "what")), 14) +
			atom.Pad(dash(screens.Cell(s, r, "cpu")), 8) + dash(screens.Cell(s, r, "memory"))
		out = append(out, atom.End(line, width))
	}
	return out
}

func dash(v page.Value) string {
	if t := atom.Show(v); t != "" {
		return t
	}
	return "–"
}
