package disk

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const (
	KeyMove  = "v"
	KeyFree  = "f"
	KeyErase = "e"
)

func title(g disk.Group) string {
	switch g {
	case disk.Misplaced:
		return "Misplaced"
	case disk.CanRebuild:
		return "Can rebuild"
	case disk.YourData:
		return "Your data"
	case disk.NeverTouched:
		return "Never touched"
	}
	return string(g)
}

type RowJSON struct {
	App    string   `json:"app"`
	Group  string   `json:"group"`
	Bytes  int64    `json:"bytes"`
	Paths  []string `json:"paths"`
	MoveTo string   `json:"move_to,omitempty"`
}

type ReadJSON struct {
	Schema   string    `json:"schema"`
	Findings []string  `json:"findings"`
	Free     int64     `json:"free_bytes"`
	Total    int64     `json:"total_bytes"`
	Rows     []RowJSON `json:"rows"`
}

func ReadContract(r disk.Reading) ReadJSON {
	out := ReadJSON{Schema: "station.disk.read/v1", Findings: []string{}, Free: r.Free, Total: r.Total, Rows: make([]RowJSON, 0, len(r.Rows))}
	for _, row := range r.Rows {
		if row.Group == disk.Misplaced {
			out.Findings = append(out.Findings, row.Paths[0]+" is misplaced")
		}
		out.Rows = append(out.Rows, RowJSON{App: row.App, Group: string(row.Group), Bytes: row.Bytes, Paths: row.Paths, MoveTo: row.MoveTo})
	}
	return out
}

func machineLabel(here page.Context) string {
	if here.Kind == string(catalog.Host) || here.Machine == "" {
		return "This Mac"
	}
	return here.Machine
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func Read(r disk.Reading, here page.Context) page.Page {
	p := page.Page{ID: "disk", Command: "disk", Context: here, Actions: Actions(r)}
	misplaced := r.Of(disk.Misplaced)
	head := page.Row{ID: "volume", Label: machineLabel(here), Mark: page.OK, Cells: []page.Value{
		page.List(page.SizeOf(r.Free, r.Total, "free"), page.Text(plural(len(misplaced), "misplaced folder"))),
	}}
	if len(misplaced) > 0 {
		head.Mark, head.Reason = page.Attention, page.Stray
		head.Hint = page.Cmd("station disk move " + misplaced[0].Paths[0])
	}
	p.Sections = append(p.Sections, page.Section{ID: "volume", Title: "Disk", Role: page.Banner, Uncounted: true, Columns: columns(), Items: []page.Item{{Row: &head}}})
	for _, g := range disk.Groups() {
		rows := r.Of(g)
		if len(rows) == 0 {
			continue
		}
		items := make([]page.Item, 0, len(rows))
		for _, row := range rows {
			pr := Row(row)
			items = append(items, page.Item{Row: &pr})
		}
		p.Sections = append(p.Sections, page.Section{ID: string(g), Title: title(g), Fold: page.Open, Columns: columns(), Items: items, Uncounted: true})
	}
	if disk.Sum(r.Of(disk.CanRebuild)) > 0 {
		p.Hints = append(p.Hints, page.Hint{Command: "station disk free", Text: "frees every Can rebuild row, nothing is lost"})
	}
	return page.Seal(p)
}

func sized(n int64, unit string) page.Value {
	v := page.Size(n)
	v.Unit = unit
	return v
}

func columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "size", Label: "size", Kind: page.KindSize, Keep: 90},
	}
}

func Row(r disk.Row) page.Row {
	cell := page.Size(r.Bytes)
	if r.Bytes < 0 {
		cell = page.Unknown()
	}
	row := page.Row{ID: string(r.Group) + "/" + r.App, Label: r.App, Mark: page.OK, Cells: []page.Value{cell}}
	if r.Group == disk.Misplaced {
		row.ID, row.Label = string(disk.Misplaced)+"/"+r.Paths[0], r.Paths[0]
		row.Mark, row.Reason = page.Attention, page.Stray
		row.Detail = page.Texts("misplaced", r.App)
		row.Hint = page.Cmd("station disk move " + r.Paths[0])
	}
	if r.Pending {
		// invariant: a row the walk has not answered shows its last-known
		// size, Muted, never a blank that a fill would move.
		row.Mark = page.Awaiting
		row.Cells = []page.Value{page.Stale(page.Size(r.Bytes))}
	}
	return row
}

func Actions(r disk.Reading) []page.Action {
	var move, free, erase []string
	for _, row := range r.Rows {
		switch row.Group {
		case disk.Misplaced:
			move = append(move, string(disk.Misplaced)+"/"+row.Paths[0])
		case disk.CanRebuild:
			free = append(free, string(row.Group)+"/"+row.App)
		case disk.YourData:
			erase = append(erase, string(row.Group)+"/"+row.App)
		case disk.NeverTouched:
		}
	}
	var out []page.Action
	if len(move) > 0 {
		out = append(out, page.Action{Key: KeyMove, Label: "move", On: page.OnRow, Rows: move, Priority: 1, Mutates: true})
	}
	if len(free) > 0 {
		out = append(out, page.Action{Key: KeyFree, Label: "free", On: page.OnRow, Rows: free, Priority: 2, Mutates: true})
	}
	if len(erase) > 0 {
		out = append(out, page.Action{Key: KeyErase, Label: "erase", On: page.OnRow, Rows: erase, Priority: 3, Mutates: true, Confirm: &page.Confirm{Yes: "erase", No: "Keep", Danger: true}})
	}
	return out
}

func EraseConfirm(app string, plan []disk.PathReport) page.Confirm {
	c := page.Confirm{Question: "Erase " + app + "?", Danger: true, Yes: "erase", No: "Keep", Type: app}
	for _, p := range plan {
		c.Lines = append(c.Lines, page.List(page.Text(title(disk.GroupOf(p.Level))), page.Path(p.Path), page.Size(p.Bytes)))
	}
	c.Lines = append(c.Lines, page.Text("its settings and keys stay"))
	return c
}

func Freed(outcomes []disk.Outcome, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(outcomes))
	var total int64
	for _, o := range outcomes {
		r := pages.Done(o.Name, o.Result, o.Reason)
		r.Cells = []page.Value{detail(o, "freed")}
		rows = append(rows, r)
		total += o.Freed
	}
	var extra []page.Section
	if len(outcomes) > 0 {
		sum := page.Row{ID: "freed", Label: "Freed", Mark: page.OK, Cells: []page.Value{page.Size(total)}, Detail: page.Text("nothing is lost")}
		extra = append(extra, page.Section{ID: "freed", Title: "Freed", Role: page.Banner, Uncounted: true, Columns: outcomeColumns(), Items: []page.Item{{Row: &sum}}})
	}
	return pages.Outcome("disk free", here, outcomeColumns(), rows, "caches to free", extra...)
}

func Erased(o disk.Outcome, here page.Context) page.Page {
	r := pages.Done(o.Name, o.Result, o.Reason)
	r.Cells = []page.Value{detail(o, "erased")}
	return pages.Outcome("disk erase", here, outcomeColumns(), []page.Row{r}, "data to erase")
}

func Moved(o disk.Outcome, here page.Context) page.Page {
	r := pages.Done(o.Name, o.Result, "")
	r.Cells = []page.Value{page.Text(o.Detail)}
	return pages.Outcome("disk move", here, outcomeColumns(), []page.Row{r}, "folders to move")
}

func outcomeColumns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "outcome", Kind: page.KindText, Keep: 50}}
}

func detail(o disk.Outcome, verb string) page.Value {
	if o.Result != catalog.StateOK {
		return page.Text(o.Detail)
	}
	return sized(o.Freed, verb)
}

type StrayJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Repair string `json:"repair"`
}

func StraysContract(strays []disk.StrayReport) []StrayJSON {
	out := make([]StrayJSON, 0, len(strays))
	for _, s := range strays {
		out = append(out, StrayJSON{Name: s.Name, Path: s.Path, Bytes: s.Bytes, Repair: s.Repair})
	}
	return out
}
