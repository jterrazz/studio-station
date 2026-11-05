package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type List struct {
	Page   page.Page
	Cursor string
	Filter string
	Open   map[string]bool
	// value: the sections the cursor never walks, as the screen that draws
	// the pane says.
	Skip func(page.Section) bool
}

func (l List) Show(p page.Page) List {
	l.Page = p
	if keys := l.keys(); !slices.Contains(keys, l.Cursor) {
		l.Cursor = ""
		if len(keys) > 0 {
			l.Cursor = keys[0]
		}
	}
	return l
}

func (l List) Move(n int) List {
	keys := l.keys()
	if len(keys) == 0 {
		return l
	}
	i := max(slices.Index(keys, l.Cursor), 0)
	l.Cursor = keys[min(max(i+n, 0), len(keys)-1)]
	return l
}

func (l List) Filtered(text string) List {
	l.Filter = text
	return l.Show(l.Page)
}

func (l List) Unfold() (List, bool) {
	if strings.Contains(l.Cursor, "/") || l.Cursor == "" || !l.folded(l.Cursor) {
		return l, false
	}
	open := make(map[string]bool, len(l.Open)+1)
	for k, v := range l.Open {
		open[k] = v
	}
	open[l.Cursor] = true
	l.Open = open
	return l, true
}

func (l List) Current() (page.Section, page.Row, bool) {
	sid, rid, isRow := strings.Cut(l.Cursor, "/")
	for _, s := range l.Page.Sections {
		if s.ID != sid || !isRow {
			continue
		}
		if r, ok := findRow(s.Items, rid); ok {
			return s, r, true
		}
	}
	return page.Section{}, page.Row{}, false
}

func (l List) Title(name string) string {
	if l.Filter == "" {
		return name
	}
	kept, all := l.count(true), l.count(false)
	return name + " (/" + l.Filter + ") " + strconv.Itoa(kept) + "/" + strconv.Itoa(all)
}

func (l List) count(filtered bool) int {
	n := 0
	var walk func(items []page.Item)
	walk = func(items []page.Item) {
		for _, it := range items {
			switch {
			case it.Row != nil:
				if !filtered || l.match(*it.Row) {
					n++
				}
			case it.Group != nil:
				if it.Group.Head.Label != "" && (!filtered || l.match(it.Group.Head)) {
					n++
				}
				walk(it.Group.Items)
			}
		}
	}
	for _, s := range l.Page.Sections {
		if s.Role != page.Banner || len(s.Columns) > 0 {
			walk(s.Items)
		}
	}
	return n
}

func (l List) Lines(s Shell, width, room int, focused bool) []string {
	st := token.New(s.Dark)
	if l.Page.Error != nil {
		return molecule.ErrorLines(st, *l.Page.Error)
	}
	var top []string
	if l.Page.Strip != nil {
		top = append(top, organism.Strip(st, *l.Page.Strip, width), "")
		room = max(room-len(top), 1)
	}
	lines := l.lines(st, width, s.Width)
	if len(lines) == 0 {
		if l.Filter != "" {
			return []string{" " + st.Muted.Render("Nothing matches /"+l.Filter+token.Sep+"esc clears it")}
		}
		return []string{" " + st.Muted.Render("Nothing here.")}
	}
	at := slices.IndexFunc(lines, func(x organism.Line) bool { return x.Key == l.Cursor })
	start := 0
	if at >= room {
		start = at - room + 1
	}
	out := make([]string, 0, room+len(top))
	out = append(out, top...)
	for i := start; i < len(lines) && len(out) < room+len(top); i++ {
		text := lines[i].Text
		if focused && lines[i].Key == l.Cursor && (lines[i].Kind == organism.LineRow || lines[i].Kind == organism.LineTitle) {
			text = s.CursorLine(text, width)
		}
		out = append(out, text)
	}
	return out
}

func (l List) Detail(dark bool, width int) []string {
	section, row, ok := l.Current()
	if !ok {
		return nil
	}
	return organism.Detail(token.New(dark), section, row, nil, width)
}

func (l List) folded(id string) bool {
	if l.Filter != "" || l.Open[id] {
		return false
	}
	for _, s := range l.Page.Sections {
		if s.ID == id {
			return organism.AutoFolded(s)
		}
	}
	return false
}

func (l List) keys() []string {
	var keys []string
	for _, x := range l.lines(token.New(true), token.SplitWidth, token.SplitWidth) {
		if x.Key != "" && (x.Kind == organism.LineTitle || x.Kind == organism.LineRow) && !slices.Contains(keys, x.Key) {
			keys = append(keys, x.Key)
		}
	}
	return keys
}

func (l List) Matches(r page.Row) bool { return l.match(r) }

func (l List) match(r page.Row) bool {
	needle := strings.ToLower(l.Filter)
	if needle == "" {
		return true
	}
	if strings.Contains(strings.ToLower(r.Label), needle) || strings.Contains(strings.ToLower(r.ID), needle) {
		return true
	}
	for _, c := range r.Cells {
		if strings.Contains(strings.ToLower(atom.Plain(c)), needle) {
			return true
		}
	}
	return false
}

func (l List) lines(st token.Styles, width, columns int) []organism.Line {
	var out []organism.Line
	for _, s := range l.Page.Sections {
		if s.Role == page.Lifted || (l.Skip != nil && l.Skip(s)) {
			continue
		}
		g := organism.Grid{Styles: st, Section: s, Width: width, Medium: molecule.TUI, Keys: molecule.NoKeys, Folded: l.folded, Match: l.match}
		body := g.Lines()
		rows := slices.ContainsFunc(body, func(x organism.Line) bool { return x.Kind == organism.LineRow && x.Key != "" })
		if l.Filter != "" && !rows {
			continue
		}
		if len(out) > 0 {
			out = append(out, organism.Line{Kind: organism.LineOther})
		}
		if s.Title != "" || len(s.Caption) > 0 {
			title := organism.CountedTitle(st, s)
			key := ""
			if organism.AutoFolded(s) {
				title, key = organism.FoldTitle(st, s, l.folded(s.ID), width), s.ID
			}
			out = append(out, organism.Line{Key: key, Kind: organism.LineTitle, Text: title})
		}
		if l.folded(s.ID) {
			continue
		}
		out = append(out, body...)
		if dropped := g.Dropped(); s.SaysDropped && len(dropped) > 0 {
			said := strconv.Itoa(columns) + " columns: " + and(dropped) + " left out"
			if more := said + token.Sep + "widen for more"; atom.Width(more) < width-token.Margin-token.Indent {
				said = more
			}
			out = append(out, organism.Line{Kind: organism.LineOther}, organism.Line{Kind: organism.LineOther, Text: atom.Spaces(token.Margin+token.Indent) + st.Muted.Render(said)})
		}
	}
	return out
}

func and(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

func findRow(items []page.Item, id string) (page.Row, bool) {
	for _, it := range items {
		switch {
		case it.Row != nil && it.Row.ID == id:
			return *it.Row, true
		case it.Group != nil && it.Group.Head.ID == id:
			return it.Group.Head, true
		case it.Group != nil:
			if r, ok := findRow(it.Group.Items, id); ok {
				return r, true
			}
		}
	}
	return page.Row{}, false
}
