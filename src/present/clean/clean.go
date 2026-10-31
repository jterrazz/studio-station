package clean

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/present"
)

type itemJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Bytes       int64  `json:"bytes"`
}

type listJSON struct {
	Schema string     `json:"schema"`
	Items  []itemJSON `json:"items"`
}

func Rows(reports []clean.Report) []present.Row {
	rows := make([]present.Row, 0, len(reports))
	for _, r := range reports {
		mark, size := present.MarkOK, present.Bytes(r.Bytes)
		if !r.Available {
			mark, size = present.MarkAbsent, "not available"
		}
		rows = append(rows, present.Row{Mark: mark, Cells: []present.Cell{
			{Text: r.Item.Name, Tone: present.Strong},
			{Text: size},
			{Text: r.Item.Description, Tone: present.Muted},
		}})
	}
	return rows
}

func List(reports []clean.Report, context string) present.Answer {
	body := listJSON{Schema: "station.clean/v1", Items: make([]itemJSON, 0, len(reports))}
	for _, r := range reports {
		body.Items = append(body.Items, itemJSON{Name: r.Item.Name, Description: r.Item.Description, Available: r.Available, Bytes: r.Bytes})
	}
	blocks := []present.Block{present.Table{Rows: Rows(reports)}}
	if len(reports) == 0 {
		blocks = []present.Block{present.Table{Rows: []present.Row{{Cells: []present.Cell{{Text: "nothing to reclaim here", Tone: present.Muted}}}}}}
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station clean",
			Context: context,
			Blocks:  blocks,
			Hints:   []string{"station clean <item...> --yes", "station clean --all --yes"},
		},
		JSON: body,
	}
}

func Outcomes(outcomes []clean.Outcome, context string) present.Answer {
	rows := make([]present.Row, 0, len(outcomes))
	for _, o := range outcomes {
		detail := o.Detail
		if o.Result == catalog.StateAbsent {
			detail = "skipped: " + detail
		}
		rows = append(rows, present.Row{Mark: present.MarkOf(o.Result), Cells: []present.Cell{
			{Text: o.Name, Tone: present.Strong},
			{Text: detail, Tone: present.Muted},
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Cells: []present.Cell{{Text: "nothing to clean", Tone: present.Muted}}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station clean",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
