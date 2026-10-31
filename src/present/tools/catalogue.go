package tools

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
)

type toolJSON struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Method   string   `json:"method"`
	Places   []string `json:"places"`
	State    string   `json:"state"`
	Version  string   `json:"version"`
	Managed  bool     `json:"managed"`
	Provides []string `json:"provides"`
	Detail   string   `json:"detail"`
}

type catalogueJSON struct {
	Schema string     `json:"schema"`
	Tools  []toolJSON `json:"tools"`
}

func Catalogue(reports []tools.Report, context string) present.Answer {
	body := catalogueJSON{Schema: "station.tools/v1", Tools: make([]toolJSON, 0, len(reports))}
	for _, r := range reports {
		body.Tools = append(body.Tools, jsonOf(r))
	}
	var blocks []present.Block
	for _, category := range catalog.Categories {
		var group []tools.Report
		for _, r := range reports {
			if r.Tool.Category == category {
				group = append(group, r)
			}
		}
		if len(group) == 0 {
			continue
		}
		blocks = append(blocks, present.Section{Title: string(category), Counts: counts(group), Blocks: []present.Block{present.Table{Rows: Rows(group)}}})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, present.Table{Rows: []present.Row{{Cells: []present.Cell{{Text: "none", Tone: present.Muted}}}}})
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station tools",
			Context: context,
			Blocks:  blocks,
			Hints:   []string{"station tools install <tool...>", "station tools upgrade"},
		},
		JSON: body,
	}
}

func jsonOf(r tools.Report) toolJSON {
	kinds := r.Tool.Places()
	places := make([]string, 0, len(kinds))
	for _, k := range kinds {
		places = append(places, string(k))
	}
	provides := append([]string{}, r.Tool.Provides...)
	detail := r.Detail
	if detail == "" {
		detail = r.Live
	}
	return toolJSON{
		Name:     r.Tool.Name,
		Category: string(r.Tool.Category),
		Method:   string(r.Method),
		Places:   places,
		State:    string(r.State),
		Version:  r.Version,
		Managed:  r.Managed,
		Provides: provides,
		Detail:   detail,
	}
}

func Rows(reports []tools.Report) []present.Row {
	rows := make([]present.Row, 0, len(reports))
	for _, r := range reports {
		detail := r.Detail
		switch {
		case r.State == catalog.StateOK && detail == "":
			detail = r.Live
		case r.State == catalog.StateAbsent && detail == "":
			detail = "station tools install " + r.Tool.Name
		}
		rows = append(rows, present.Row{Mark: present.MarkOf(r.State), Cells: []present.Cell{
			{Text: r.Tool.Name, Tone: present.Strong},
			{Text: r.Version},
			{Text: string(r.Method), Tone: present.Muted},
			{Text: detail, Tone: present.Muted},
		}})
	}
	return rows
}

func Outcomes(verb string, outcomes []tools.Outcome, context string) present.Answer {
	rows := make([]present.Row, 0, len(outcomes))
	for _, o := range outcomes {
		rows = append(rows, present.Row{Mark: present.MarkOf(o.Result), Cells: []present.Cell{
			{Text: o.Name, Tone: present.Strong},
			{Text: o.Detail, Tone: present.Muted},
		}})
		for _, s := range o.Steps {
			rows = append(rows, present.Row{Mark: present.MarkOf(s.Result), Cells: []present.Cell{
				{Text: s.Item, Tone: present.Strong},
				{Text: s.Detail, Tone: present.Muted},
			}})
		}
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Cells: []present.Cell{{Text: "nothing to " + verb, Tone: present.Muted}}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station tools " + verb,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func counts(reports []tools.Report) []present.Count {
	n := map[present.Mark]int{}
	for _, r := range reports {
		n[present.MarkOf(r.State)]++
	}
	out := make([]present.Count, 0, len(n))
	for _, m := range []present.Mark{present.MarkOK, present.MarkDegraded, present.MarkAbsent, present.MarkBroken} {
		if n[m] > 0 {
			out = append(out, present.Count{Mark: m, N: n[m]})
		}
	}
	return out
}
