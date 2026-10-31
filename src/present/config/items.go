package config

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/present"
)

type ItemJSON struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Places   []string `json:"places"`
	State    string   `json:"state"`
	Detail   string   `json:"detail"`
}

type itemsJSON struct {
	Schema string     `json:"schema"`
	Items  []ItemJSON `json:"items"`
}

func ItemObject(r config.Report) ItemJSON {
	places := make([]string, len(r.Item.Places))
	for i, p := range r.Item.Places {
		places[i] = string(p)
	}
	return ItemJSON{Name: r.Item.Name, Category: string(r.Item.Category), Places: places, State: string(r.State), Detail: r.Detail}
}

func categoryOrder() []catalog.ConfigCategory {
	return []catalog.ConfigCategory{
		catalog.CategoryShell, catalog.CategoryTerminal, catalog.CategorySecurity, catalog.CategoryEditor,
		catalog.CategorySystem, catalog.CategoryHeadless, catalog.CategoryNetwork,
	}
}

func Items(reports []config.Report, context string) present.Answer {
	body := itemsJSON{Schema: "station.config/v1", Items: make([]ItemJSON, 0, len(reports))}
	for _, r := range reports {
		body.Items = append(body.Items, ItemObject(r))
	}
	var blocks []present.Block
	for _, category := range categoryOrder() {
		var group []config.Report
		for _, r := range reports {
			if r.Item.Category == category {
				group = append(group, r)
			}
		}
		if len(group) == 0 {
			continue
		}
		blocks = append(blocks, present.Section{Title: string(category), Counts: Counts(group), Blocks: []present.Block{present.Table{Rows: Rows(group)}}})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, present.Table{Rows: []present.Row{{Cells: []present.Cell{{Text: "none", Tone: present.Muted}}}}})
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station config",
			Context: context,
			Blocks:  blocks,
			Hints:   []string{"station config install <item...>", "station config uninstall <item...>"},
		},
		JSON: body,
	}
}

func Rows(reports []config.Report) []present.Row {
	rows := make([]present.Row, 0, len(reports))
	for _, r := range reports {
		rows = append(rows, present.Row{Mark: present.MarkOf(r.State), Cells: []present.Cell{
			{Text: r.Item.Name, Tone: present.Strong},
			{Text: r.Item.Description},
			{Text: detail(r), Tone: present.Muted},
		}})
	}
	return rows
}

// value: an absent item says what to run, and one whose tool is missing says
// which tool first, so the next step is always on the row.
func detail(r config.Report) string {
	switch {
	case r.State == catalog.StateAbsent && r.Detail == "" && !r.Applies:
		return "needs " + r.Item.Requires + ": station tools install " + r.Item.Requires
	case r.State == catalog.StateAbsent && r.Detail == "":
		return "station config install " + r.Item.Name
	}
	return r.Detail
}

func Counts(reports []config.Report) []present.Count {
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
