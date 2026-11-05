package settings

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	"github.com/jterrazz/studio-station/src/settings"
)

func Matrix(m settings.Matrix, now time.Time, all bool, label func(string) string, here page.Context) page.Page {
	// invariant: the machines first, each read now, kept from its last read
	// or said silent, and the servers that are no column; then what needs a
	// look, with its gesture; then one row per setting and one column per
	// machine, a group's rows that need no look folded on one line unless
	// every row is unfolded.
	p := page.Page{ID: ID, Command: "settings --all-machines", Context: here}
	p.Sections = append(p.Sections, machinesSection(m, now, label))
	if look := lookSection(m, now, label); len(look.Items) > 0 {
		p.Sections = append(p.Sections, look)
	}
	for _, g := range []struct {
		group settings.Group
		id    string
		title string
	}{{settings.SetByStation, "set", "Set by station"}, {settings.AlwaysOnMac, "always-on-mac", "Always-on Mac"}, {settings.Checked, "checked", "Checked"}} {
		if s, ok := groupSection(m, g.group, g.id, g.title, all, label); ok {
			p.Sections = append(p.Sections, s)
		}
	}
	if !all {
		p.Hints = []page.Hint{{Command: "station settings --all-machines --all", Text: "every row"}}
	}
	return page.Seal(p)
}

func machinesSection(m settings.Matrix, now time.Time, label func(string) string) page.Section {
	looks := 0
	for _, r := range m.Rows {
		for _, c := range r.Cells {
			if c.Look() {
				looks++
			}
		}
	}
	caption := []page.Value{page.Text(strconv.Itoa(len(m.Rows)) + " settings"), page.Text(strconv.Itoa(len(m.Columns)) + " machines")}
	if looks > 0 {
		caption = append(caption, page.Text(strconv.Itoa(looks)+" need a look"))
	}
	cols := []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "kind", Kind: page.KindText, Keep: 40}, {Key: "state", Kind: page.KindText, Keep: 60}}
	s := page.Section{ID: "machines", Title: "Machines", Caption: caption, Columns: cols, Uncounted: true}
	for _, c := range m.Columns {
		mark, reason, words := standing(c, now)
		row := page.Row{ID: c.Name, Label: c.Name, Short: heading(c.Place, label), Mark: mark, Reason: reason, Cells: []page.Value{page.Text(kindWord(c.Place)), page.Texts(words...)}}
		s.Items = append(s.Items, page.Item{Row: &row})
	}
	for _, name := range m.Servers {
		row := page.Row{ID: name, Label: name, Short: label(name), Mark: page.NotForThisMachine, Cells: []page.Value{page.Text("Server"), page.Text("not checked: station does not manage its settings")}}
		s.Items = append(s.Items, page.Item{Row: &row})
	}
	if len(s.Items) == 0 {
		s.Items = []page.Item{{Empty: &page.Empty{Noun: "machines"}}}
	}
	return s
}

// value: how a machine's column stands: read now, its last read kept, or
// why there is none.
func standing(c settings.Column, now time.Time) (page.Mark, page.Reason, []string) {
	switch {
	case c.Away == settings.Silent && c.Mac:
		return page.Broken, page.Unreachable, []string{"does not answer", "last answer " + pages.LastAnswer(c.Answered, now)}
	case c.Away == settings.Silent:
		return page.Broken, page.Unreachable, []string{"its Mac does not answer"}
	case c.Why != "":
		return page.Broken, page.Unreachable, []string{c.Why}
	case c.Away == settings.Stopped && c.Known():
		return page.Absent, page.Stopped, []string{"stopped", "last read " + ago(c.Read, now)}
	case c.Away == settings.Stopped:
		return page.NotKnown, page.Stopped, []string{"stopped", "not read since the engine started"}
	}
	return page.OK, "", []string{"read"}
}

func heading(p settings.Place, label func(string) string) string {
	if p.Here {
		return "This Mac"
	}
	return label(p.Name)
}

func kindWord(p settings.Place) string {
	if p.Mac {
		return "Mac"
	}
	return "Workspace"
}

func lookSection(m settings.Matrix, now time.Time, label func(string) string) page.Section {
	s := page.Section{ID: "needs-a-look", Title: "Needs a look", Columns: []page.Column{{Key: "machine", Kind: page.KindText, Keep: 100}, {Key: "setting", Kind: page.KindText, Keep: 100}, {Key: "state", Kind: page.KindText, Keep: 60}}}
	for i, c := range m.Columns {
		for _, mr := range m.Rows {
			if !mr.Cells[i].Look() {
				continue
			}
			r, _ := c.Row(mr.Name)
			one := Row(r, now)
			row := page.Row{ID: c.Name + "/" + mr.Name, Label: c.Name, Short: heading(c.Place, label), Mark: one.Mark, Reason: one.Reason, Cells: append([]page.Value{page.Text(mr.Name)}, one.Cells...)}
			switch {
			case c.Away == settings.Stopped:
				row.Hint = page.Cmd("station machines start " + c.Name)
			case one.Hint != nil:
				hint := *one.Hint
				hint.Key = ""
				if !hint.External && !c.Here {
					hint.Command += " --on " + c.Name
				}
				row.Hint = &hint
			}
			s.Items = append(s.Items, page.Item{Row: &row})
		}
	}
	return s
}

func groupSection(m settings.Matrix, g settings.Group, id, title string, all bool, label func(string) string) (page.Section, bool) {
	cols := make([]page.Column, 0, len(m.Columns)+2)
	cols = append(cols, page.Column{Key: "name", Kind: page.KindText, Keep: 100})
	for _, c := range m.Columns {
		cols = append(cols, page.Column{Key: c.Name, Label: heading(c.Place, label), Kind: page.KindText, Keep: 20})
	}
	cols = append(cols, page.Column{Key: "ok", Label: "OK", Kind: page.KindText, Keep: 20})
	s := page.Section{ID: id, Title: title, Columns: cols, Uncounted: true}
	var rest []settings.MatrixRow
	for _, r := range m.Rows {
		if r.Group != g {
			continue
		}
		if !all && !looks(r) {
			rest = append(rest, r)
			continue
		}
		row := page.Row{ID: r.Name, Label: r.Name, Mark: rowMark(r)}
		if row.Mark == page.Attention {
			row.Reason = page.Differs
		}
		for _, c := range r.Cells {
			row.Cells = append(row.Cells, page.Marked(cellMark(c)))
		}
		row.Cells = append(row.Cells, okOf(r.OK, r.Of))
		s.Items = append(s.Items, page.Item{Row: &row})
	}
	if len(rest) > 0 {
		folded := foldedRow(m, rest, len(s.Items) > 0)
		s.Items = append(s.Items, page.Item{Row: &folded})
	}
	return s, len(s.Items) > 0
}

func looks(r settings.MatrixRow) bool {
	for _, c := range r.Cells {
		if c.Look() {
			return true
		}
	}
	return false
}

// value: the rows that need no look, one line: each machine's OK count of
// the settings that live there, and theirs together.
func foldedRow(m settings.Matrix, rows []settings.MatrixRow, more bool) page.Row {
	name := strconv.Itoa(len(rows)) + " settings"
	if more {
		name = strconv.Itoa(len(rows)) + " more"
	}
	row := page.Row{ID: "folded", Label: name}
	okAll, ofAll, lives := 0, 0, false
	var marks []page.Mark
	for i, c := range m.Columns {
		ok, of := 0, 0
		var here []page.Mark
		for _, r := range rows {
			if r.Cells[i] != settings.Cell(settings.StateNotForThisMachine) {
				here = append(here, cellMark(r.Cells[i]))
			}
			if r.Counts[i] {
				of++
				if r.Cells[i].OK() {
					ok++
				}
			}
		}
		okAll, ofAll, lives = okAll+ok, ofAll+of, lives || (c.Known() && len(here) > 0)
		mark := worst(here)
		if c.Known() {
			marks = append(marks, mark)
		}
		switch {
		case !c.Known():
			row.Cells = append(row.Cells, page.Marked(page.NotKnown))
		case of == 0:
			row.Cells = append(row.Cells, page.Marked(mark))
		default:
			row.Cells = append(row.Cells, page.Labeled(string(mark), page.Text(strconv.Itoa(ok)+"/"+strconv.Itoa(of))))
		}
	}
	row.Mark = worst(marks)
	if !lives {
		row.Mark = page.NotForThisMachine
	}
	row.Cells = append(row.Cells, okOf(okAll, ofAll))
	return row
}

// value: the mark that says most of a few cells: one not applied, then
// one off here, then one unknown, else OK; none is not for the machine.
func worst(marks []page.Mark) page.Mark {
	if len(marks) == 0 {
		return page.NotForThisMachine
	}
	for _, w := range []page.Mark{page.Absent, page.OffHere, page.NotKnown} {
		for _, m := range marks {
			if m == w {
				return w
			}
		}
	}
	return page.OK
}

func okOf(ok, of int) page.Value {
	if of == 0 {
		return page.Marked(page.NotForThisMachine)
	}
	return page.Text(strconv.Itoa(ok) + "/" + strconv.Itoa(of))
}

func rowMark(r settings.MatrixRow) page.Mark {
	switch {
	case looks(r):
		return page.Attention
	case r.Of == 0:
		return page.NotForThisMachine
	case r.OK == r.Of:
		return page.OK
	}
	return page.Absent
}

func cellMark(c settings.Cell) page.Mark {
	switch c {
	case settings.Cell(settings.StateApplied), settings.Cell(settings.AsExpected):
		return page.OK
	case settings.Cell(settings.StateChangedSince), settings.Cell(settings.NeedsFix):
		return page.Attention
	case settings.Cell(settings.StateNotApplied):
		return page.Absent
	case settings.Cell(settings.StateOffHere):
		return page.OffHere
	case settings.Cell(settings.StateNotForThisMachine):
		return page.NotForThisMachine
	case settings.CellUnknown:
	}
	return page.NotKnown
}

type MatrixJSON struct {
	Schema   string          `json:"schema"`
	Machine  MachineJSON     `json:"machine"`
	Findings []string        `json:"findings"`
	Columns  []string        `json:"columns"`
	Skipped  []SkippedJSON   `json:"skipped"`
	Rows     []MatrixRowJSON `json:"rows"`
}

type MachineJSON struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type SkippedJSON struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

type MatrixRowJSON struct {
	Name  string   `json:"name"`
	Group string   `json:"group"`
	Cells []string `json:"cells"`
	OK    int      `json:"ok"`
	Of    int      `json:"of"`
}

func contractKind(kind string) string {
	if kind == string(catalog.Host) {
		return "mac"
	}
	return "workspace"
}

func MatrixContract(m settings.Matrix, now time.Time, here page.Context) MatrixJSON {
	// invariant: a machine that could not be read now is a finding, in the
	// words of its row in Machines.
	body := MatrixJSON{Schema: "station.settings.matrix/v1", Machine: MachineJSON{Name: here.Machine, Kind: contractKind(here.Kind)}, Findings: []string{}, Columns: []string{}, Skipped: []SkippedJSON{}, Rows: []MatrixRowJSON{}}
	for _, c := range m.Columns {
		body.Columns = append(body.Columns, c.Name)
		if mark, _, words := standing(c, now); mark != page.OK {
			body.Findings = append(body.Findings, c.Name+": "+strings.Join(words, ", "))
		}
	}
	for _, s := range m.Servers {
		body.Skipped = append(body.Skipped, SkippedJSON{Name: s, Why: "server"})
	}
	for _, r := range m.Rows {
		cells := make([]string, len(r.Cells))
		for i, c := range r.Cells {
			cells[i] = string(c)
		}
		body.Rows = append(body.Rows, MatrixRowJSON{Name: r.Name, Group: string(r.Group), Cells: cells, OK: r.OK, Of: r.Of})
	}
	return body
}
