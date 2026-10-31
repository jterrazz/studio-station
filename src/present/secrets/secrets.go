package secrets

import (
	"slices"
	"sort"
	"strings"

	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/present"
)

type listJSON struct {
	Schema   string              `json:"schema"`
	LendList []string            `json:"lend_list"`
	Lent     map[string][]string `json:"lent"`
	Kept     []string            `json:"kept"`
}

func List(r secrets.Report, context string) present.Answer {
	body := listJSON{Schema: "station.secrets/v1", LendList: nonNil(r.LendList), Lent: map[string][]string{}, Kept: nonNil(r.Kept)}
	for vm, names := range r.Lent {
		body.Lent[vm] = names
	}
	blocks := []present.Block{
		present.Section{Title: "Lend list", Blocks: []present.Block{present.Table{Rows: lendRows(r)}}},
		present.Section{Title: "Kept", Blocks: []present.Block{present.Table{Rows: keptRows(r.Kept)}}},
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station secrets",
			Context: context,
			Blocks:  blocks,
			Hints:   []string{"station secrets allow <name...>", "station secrets lend --to <vm>"},
		},
		JSON: body,
	}
}

func lendRows(r secrets.Report) []present.Row {
	if len(r.LendList) == 0 {
		return []present.Row{{Cells: []present.Cell{{Text: "none", Tone: present.Muted}}}}
	}
	rows := make([]present.Row, 0, len(r.LendList))
	for _, name := range r.LendList {
		var holders []string
		for vm, names := range r.Lent {
			if slices.Contains(names, name) {
				holders = append(holders, vm)
			}
		}
		sort.Strings(holders)
		detail := "not lent"
		mark := present.MarkAbsent
		if len(holders) > 0 {
			detail = "lent to " + strings.Join(holders, ", ")
			mark = present.MarkOK
		}
		rows = append(rows, present.Row{Mark: mark, Cells: []present.Cell{
			{Text: name, Tone: present.Strong},
			{Text: detail, Tone: present.Muted},
		}})
	}
	return rows
}

func keptRows(kept []string) []present.Row {
	if len(kept) == 0 {
		return []present.Row{{Cells: []present.Cell{{Text: "none", Tone: present.Muted}}}}
	}
	rows := make([]present.Row, 0, len(kept))
	for _, account := range kept {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: account, Tone: present.Strong},
			{Text: "never leaves this host", Tone: present.Muted},
		}})
	}
	return rows
}

func Change(verb string, names []string, recall secrets.RecallReport, context string) present.Answer {
	rows := make([]present.Row, 0, len(names))
	for _, name := range names {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: name, Tone: present.Strong},
			{Text: changeText(verb), Tone: present.Muted},
		}})
	}
	rows = append(rows, recallRows(recall)...)
	return present.Answer{Doc: present.Document{
		Command: "station secrets " + verb,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func Lent(r secrets.LendReport, context string) present.Answer {
	rows := make([]present.Row, 0, len(r.Names))
	for _, name := range r.Names {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: name, Tone: present.Strong},
			{Text: "lent to " + r.To, Tone: present.Muted},
		}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station secrets lend",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
		Hints:   []string{"station secrets recall --from " + r.To},
	}}
}

func Recalled(r secrets.RecallReport, context string) present.Answer {
	rows := recallRows(r)
	if len(rows) == 0 {
		rows = append(rows, present.Row{Cells: []present.Cell{{Text: "nothing to recall", Tone: present.Muted}}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station secrets recall",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func recallRows(r secrets.RecallReport) []present.Row {
	var rows []present.Row
	vms := make([]string, 0, len(r.From))
	for vm := range r.From {
		vms = append(vms, vm)
	}
	sort.Strings(vms)
	for _, vm := range vms {
		for _, name := range r.From[vm] {
			rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
				{Text: name, Tone: present.Strong},
				{Text: "recalled from " + vm, Tone: present.Muted},
			}})
		}
	}
	for _, w := range r.Warnings {
		rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
			{Text: "recall", Tone: present.Strong},
			{Text: w, Tone: present.Muted},
		}})
	}
	return rows
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func changeText(verb string) string {
	if verb == "allow" {
		return "on the lend list"
	}
	return "off the lend list"
}
