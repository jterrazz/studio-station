package storage

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/present"
)

type pathJSON struct {
	Path  string `json:"path"`
	Level string `json:"level"`
	Bytes int64  `json:"bytes"`
}

type rowJSON struct {
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
	Paths          []pathJSON `json:"paths"`
}

type StrayJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Repair string `json:"repair"`
}

type listJSON struct {
	Schema string      `json:"schema"`
	Rows   []rowJSON   `json:"rows"`
	Strays []StrayJSON `json:"strays"`
}

func List(r storage.Report, context string) present.Answer {
	return present.Answer{
		Doc: present.Document{
			Command: "station storage",
			Context: context,
			Blocks:  Sections(r),
			Hints:   []string{"station storage clean frees " + present.Bytes(cacheTotal(r)), "station storage reset <tool> deletes a tool's data, asks first"},
		},
		JSON: listJSON{Schema: "station.storage/v1", Rows: rowsJSON(r.Rows), Strays: Strays(r.Strays)},
	}
}

func Strays(strays []storage.StrayReport) []StrayJSON {
	out := make([]StrayJSON, 0, len(strays))
	for _, s := range strays {
		out = append(out, StrayJSON{Name: s.Name, Path: s.Path, Bytes: s.Bytes, Repair: s.Repair})
	}
	return out
}

func rowsJSON(rows []storage.Row) []rowJSON {
	out := make([]rowJSON, 0, len(rows))
	for _, r := range rows {
		paths := make([]pathJSON, 0, len(r.Paths))
		for _, p := range r.Paths {
			paths = append(paths, pathJSON{Path: p.Path, Level: string(p.Level), Bytes: p.Bytes})
		}
		out = append(out, rowJSON{
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

func cacheTotal(r storage.Report) int64 {
	var total int64
	for _, row := range r.Rows {
		total += row.Bytes[catalog.LevelCache]
	}
	for _, s := range r.Strays {
		total += s.Bytes
	}
	return total
}

type Group struct {
	Title string
	Rows  []storage.Row
}

func Groups(r storage.Report) []Group {
	order := make([]string, 0, 1+len(catalog.Categories))
	order = append(order, storage.StationCategory)
	for _, c := range catalog.Categories {
		order = append(order, string(c))
	}
	var out []Group
	for _, title := range order {
		g := Group{Title: title}
		for _, row := range r.Rows {
			if row.Entry.Category == title {
				g.Rows = append(g.Rows, row)
			}
		}
		if len(g.Rows) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func Sections(r storage.Report) []present.Block {
	var blocks []present.Block
	for _, g := range Groups(r) {
		blocks = append(blocks, present.Section{Title: g.Title, Blocks: []present.Block{present.Table{Rows: Rows(g.Rows)}}})
	}
	if len(r.Strays) > 0 {
		blocks = append(blocks, present.Section{Title: "Strays", Blocks: []present.Block{present.Table{Rows: StrayRows(r.Strays)}}})
	}
	return blocks
}

func Rows(rows []storage.Row) []present.Row {
	out := make([]present.Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, Row(r))
	}
	return out
}

func Row(r storage.Row) present.Row {
	cells := []present.Cell{
		{Text: r.Entry.Name, Tone: present.Strong},
		{Text: present.Bytes(r.Bytes[catalog.LevelCache])},
		{Text: Detail(r), Tone: present.Muted},
	}
	if len(r.Strays) > 0 {
		cells = append(cells, present.Cell{Text: r.Strays[0].Repair, Tone: present.Muted})
	}
	return present.Row{Mark: present.MarkOf(r.State), Cells: cells}
}

func Detail(r storage.Row) string {
	var parts []string
	for _, part := range []struct {
		name  string
		bytes int64
	}{
		{"data", r.Bytes[catalog.LevelData]},
		{"kept", r.Bytes[catalog.LevelKept]},
		{"installed", r.Install},
	} {
		if part.bytes > 0 {
			parts = append(parts, part.name+" "+present.Bytes(part.bytes))
		}
	}
	return strings.Join(parts, " · ")
}

func StrayRows(strays []storage.StrayReport) []present.Row {
	out := make([]present.Row, 0, len(strays))
	for _, s := range strays {
		out = append(out, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
			{Text: s.Name, Tone: present.Strong},
			{Text: s.Path},
			{Text: present.Bytes(s.Bytes)},
			{Text: s.Repair, Tone: present.Muted},
		}})
	}
	return out
}

func Outcomes(verb string, outcomes []storage.Outcome, context string) present.Answer {
	rows := make([]present.Row, 0, len(outcomes))
	for _, o := range outcomes {
		cells := []present.Cell{{Text: o.Name, Tone: present.Strong}, {Text: OutcomeDetail(o), Tone: present.Muted}}
		if o.Reason != "" {
			cells = append(cells, present.Cell{Text: o.Reason, Tone: present.Muted})
		}
		rows = append(rows, present.Row{Mark: present.MarkOf(o.Result), Cells: cells})
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Cells: []present.Cell{{Text: "nothing to " + verb, Tone: present.Muted}}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station storage " + verb,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func OutcomeDetail(o storage.Outcome) string {
	if o.Result != catalog.StateOK {
		return o.Detail
	}
	freed := present.Bytes(o.Freed) + " freed"
	if o.Reset {
		return "reset · " + freed
	}
	return freed
}

func Size(n int64) string { return present.Bytes(n) }
