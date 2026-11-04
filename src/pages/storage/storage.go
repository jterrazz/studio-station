package storage

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const (
	KeyClean = "c"
	KeyReset = "x"
)

type PathJSON struct {
	Path  string `json:"path"`
	Level string `json:"level"`
	Bytes int64  `json:"bytes"`
}

type RowJSON struct {
	Name           string     `json:"name"`
	Category       string     `json:"category"`
	Installed      bool       `json:"installed"`
	Mark           string     `json:"mark"`
	CacheBytes     int64      `json:"cache_bytes"`
	DataBytes      int64      `json:"data_bytes"`
	ConfigBytes    int64      `json:"config_bytes"`
	KeptBytes      int64      `json:"kept_bytes"`
	InstalledBytes int64      `json:"installed_bytes"`
	Clean          bool       `json:"clean"`
	Reset          bool       `json:"reset"`
	Paths          []PathJSON `json:"paths"`
}

type StrayJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Repair string `json:"repair"`
}

type ListJSON struct {
	Schema string      `json:"schema"`
	Rows   []RowJSON   `json:"rows"`
	Strays []StrayJSON `json:"strays"`
}

func ListContract(r storage.Report) ListJSON {
	return ListJSON{Schema: "station.storage/v1", Rows: rowsJSON(r.Rows), Strays: StraysContract(r.Strays)}
}

func StraysContract(strays []storage.StrayReport) []StrayJSON {
	out := make([]StrayJSON, 0, len(strays))
	for _, s := range strays {
		out = append(out, StrayJSON{Name: s.Name, Path: s.Path, Bytes: s.Bytes, Repair: s.Repair})
	}
	return out
}

func rowsJSON(rows []storage.Row) []RowJSON {
	out := make([]RowJSON, 0, len(rows))
	for _, r := range rows {
		paths := make([]PathJSON, 0, len(r.Paths))
		for _, p := range r.Paths {
			paths = append(paths, PathJSON{Path: p.Path, Level: string(p.Level), Bytes: p.Bytes})
		}
		out = append(out, RowJSON{
			Name:           r.Entry.Name,
			Category:       r.Entry.Category,
			Installed:      r.Installed,
			Mark:           string(r.State),
			CacheBytes:     r.Bytes[catalog.LevelCache],
			DataBytes:      r.Bytes[catalog.LevelData],
			ConfigBytes:    r.Bytes[catalog.LevelConfig],
			KeptBytes:      r.Bytes[catalog.LevelKept],
			InstalledBytes: r.Install,
			Clean:          len(r.Entry.Storage.Clean) > 0 || len(r.Entry.Storage.Strays) > 0,
			Reset:          len(r.Entry.Storage.Reset) > 0,
			Paths:          paths,
		})
	}
	return out
}

func order() []string {
	out := make([]string, 0, 1+len(catalog.Categories))
	out = append(out, storage.StationCategory)
	for _, c := range catalog.Categories {
		out = append(out, string(c))
	}
	return out
}

func List(r storage.Report, here page.Context) page.Page {
	p := page.Page{
		ID:      "storage",
		Command: "storage",
		Context: here,
		Hints: []page.Hint{
			{Command: "station storage clean <tool...>"},
			{Command: "station storage reset <tool...>", Text: "asks first"},
		},
		Actions: Actions(r),
	}
	for _, title := range order() {
		var items []page.Item
		for _, row := range r.Rows {
			if row.Entry.Category == title {
				pr := Row(row)
				items = append(items, page.Item{Row: &pr})
			}
		}
		if len(items) > 0 {
			p.Sections = append(p.Sections, page.Section{ID: sectionID(title), Title: title, Fold: page.Open, Columns: Columns(), Items: items, Uncounted: true})
		}
	}
	if len(r.Strays) > 0 {
		items := make([]page.Item, 0, len(r.Strays))
		for i, s := range r.Strays {
			row := page.Row{
				ID: s.Name + "/" + strconv.Itoa(i), Label: s.Name, Mark: page.Attention, Reason: page.Stray,
				Cells: []page.Value{page.Path(s.Path), page.Size(s.Bytes)},
			}
			if s.Repair != "" {
				row.Hint = page.Cmd(s.Repair)
			}
			items = append(items, page.Item{Row: &row})
		}
		p.Sections = append(p.Sections, page.Section{ID: "strays", Title: "Strays", Fold: page.Open, Columns: StrayColumns(), Items: items, Uncounted: true})
	}
	if len(p.Sections) == 0 {
		p.Sections = []page.Section{{ID: "storage", Title: "Storage", Role: page.Banner, Columns: Columns(),
			Items: []page.Item{{Empty: &page.Empty{Noun: "data stored", Hint: page.Cmd("station apps install <app>")}}}}}
	}
	return page.Seal(p)
}

func sectionID(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	return strings.Join(words, "-")
}

func Columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "cache", Label: "cache", Kind: page.KindSize, Keep: 90},
		{Key: "levels", Kind: page.KindLevels, Keep: 80, Levels: []string{"data", "kept", "installed"}},
	}
}

func StrayColumns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "path", Label: "path", Kind: page.KindPath, Keep: 60},
		{Key: "size", Label: "size", Kind: page.KindSize, Keep: 90},
	}
}

func Actions(r storage.Report) []page.Action {
	var clean, reset []string
	for _, row := range r.Rows {
		if len(row.Entry.Storage.Clean) > 0 || len(row.Entry.Storage.Strays) > 0 {
			clean = append(clean, row.Entry.Name)
		}
		if len(row.Entry.Storage.Reset) > 0 {
			reset = append(reset, row.Entry.Name)
		}
	}
	var out []page.Action
	if len(clean) > 0 {
		out = append(out, page.Action{Key: KeyClean, Label: "clean", On: page.OnRow, Rows: clean, Priority: 1, Mutates: true})
	}
	if len(reset) > 0 {
		out = append(out, page.Action{Key: KeyReset, Label: "reset", On: page.OnRow, Rows: reset, Priority: 2, Mutates: true, Confirm: &page.Confirm{Yes: "reset", No: "Keep", Danger: true}})
	}
	return out
}

func Row(r storage.Row) page.Row {
	row := page.Row{ID: r.Entry.Name, Label: r.Entry.Name, Mark: pages.MarkOf(r.State)}
	if r.Pending {
		// invariant: a row the walk has not answered shows its last-known size,
		// Muted, or nothing when it has none.
		row.Mark = page.Awaiting
		last := page.Pending()
		if b := r.Bytes[catalog.LevelCache]; b > 0 {
			last = page.Stale(page.Size(b))
		}
		row.Cells = []page.Value{last}
		return row
	}
	row.Cells = []page.Value{size(r.Bytes[catalog.LevelCache]), levels(r)}
	if len(r.Strays) > 0 && r.Strays[0].Repair != "" {
		row.Hint = page.Cmd(r.Strays[0].Repair)
	}
	switch row.Mark {
	case page.Attention:
		row.Reason = page.Stray
		if r.Unknown {
			row.Reason = page.Late
		}
	case page.Broken:
		row.Reason = page.Failing
	case page.OK, page.Absent, page.Awaiting, page.OffHere, page.NotForThisMachine, page.NotKnown, page.Unjudged:
	}
	return row
}

func levels(r storage.Row) page.Value {
	if r.Unknown {
		return page.Text("not sized within " + strconv.Itoa(int(r.Within.Round(time.Second)/time.Second)) + " s")
	}
	var parts []page.Value
	for _, part := range []struct {
		name  string
		bytes int64
	}{
		{"data", r.Bytes[catalog.LevelData]},
		{"kept", r.Bytes[catalog.LevelKept]},
		{"installed", r.Install},
	} {
		if part.bytes > 0 {
			parts = append(parts, page.Labeled(part.name, page.Size(part.bytes)))
		}
	}
	if len(parts) == 0 {
		return page.None()
	}
	return page.Levels(parts...)
}

func size(n int64) page.Value {
	if n < 0 {
		return page.Unknown()
	}
	return page.Size(n)
}

func ResetConfirm(names []string, plan []storage.PathReport) page.Confirm {
	c := page.Confirm{Question: "Reset " + strings.Join(names, ", ") + "?", Danger: true, Yes: "reset", No: "Keep"}
	for _, p := range plan {
		if p.Level != catalog.LevelCache && p.Level != catalog.LevelData {
			continue
		}
		c.Lines = append(c.Lines, page.List(page.Text(string(p.Level)), page.Path(p.Path), size(p.Bytes)))
	}
	c.Lines = append(c.Lines, page.Text("installed files and kept files stay"))
	return c
}

func Outcomes(verb string, outcomes []storage.Outcome, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(outcomes))
	for _, o := range outcomes {
		r := pages.Done(o.Name, o.Result, o.Reason)
		r.Cells = []page.Value{OutcomeDetail(o)}
		rows = append(rows, r)
	}
	return pages.Outcome("storage "+verb, here, []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "outcome", Kind: page.KindText, Keep: 50}}, rows, nounOf(verb))
}

func nounOf(verb string) string {
	if verb == "reset" {
		return "data to reset"
	}
	return "caches to clean"
}

func OutcomeDetail(o storage.Outcome) page.Value {
	if o.Result != catalog.StateOK {
		return page.Text(o.Detail)
	}
	freed := page.Size(o.Freed)
	freed.Unit = "freed"
	if o.Reset {
		return page.List(page.Text("reset"), freed)
	}
	return freed
}
