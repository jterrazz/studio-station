package tools

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
)

func Rows(reports []tools.Report) []present.Row {
	rows := make([]present.Row, 0, len(reports))
	for _, r := range reports {
		if r.Pending {
			rows = append(rows, present.Row{Mark: present.MarkPending, Cells: []present.Cell{
				{Text: r.Tool.Name, Tone: present.Strong}, {Text: ""}, {Text: method(r), Tone: present.Muted}, {Text: "", Tone: present.Muted},
			}})
			continue
		}
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
			{Text: method(r), Tone: present.Muted},
			{Text: detail, Tone: present.Muted},
		}})
	}
	return rows
}

func method(r tools.Report) string {
	if r.Tool.System {
		return string(r.Method) + " · system"
	}
	return string(r.Method)
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

func Counts(reports []tools.Report) []present.Count {
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
