package secrets

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

type ListJSON struct {
	Schema   string              `json:"schema"`
	LendList []string            `json:"lend_list"`
	Lent     map[string][]string `json:"lent"`
	Kept     []string            `json:"kept"`
}

func ListContract(r secrets.Report) ListJSON {
	body := ListJSON{Schema: "station.secrets/v1", LendList: nonNil(r.LendList), Lent: map[string][]string{}, Kept: nonNil(r.Kept)}
	for vm, names := range r.Lent {
		body.Lent[vm] = names
	}
	return body
}

func columns() []page.Column { return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}} }

func Section(r secrets.Report) page.Section {
	// invariant: one Secrets section holds the Lend list and Kept groups on
	// every surface (§13.5, A19); the pipe keeps them the two titled sections
	// its records always were.
	s := pages.Nest(page.Section{ID: "secrets", Title: "Secrets", Uncounted: true, PipeGroups: true}, []page.Section{
		{ID: "lend-list", Title: "Lend list", Columns: columns(), Items: lendItems(r)},
		{ID: "kept", Title: "Kept", Columns: columns(), Items: keptItems(r.Kept)},
	})
	s.Fold = ""
	return s
}

func List(r secrets.Report, here page.Context) page.Page {
	return page.Seal(page.Page{
		ID: "secrets", Command: "secrets", Context: here, Sections: []page.Section{Section(r)},
		Hints: []page.Hint{*page.Cmd("station secrets allow <name...>"), *page.Cmd("station secrets lend --to <vm>")},
	})
}

func lendItems(r secrets.Report) []page.Item {
	if len(r.LendList) == 0 {
		return []page.Item{{Empty: &page.Empty{Noun: "lent secrets", Hint: page.Cmd("station secrets allow <name>")}}}
	}
	items := make([]page.Item, 0, len(r.LendList))
	for _, name := range r.LendList {
		var holders []string
		for vm, names := range r.Lent {
			if slices.Contains(names, name) {
				holders = append(holders, vm)
			}
		}
		sort.Strings(holders)
		row := page.Row{ID: name, Label: name, Mark: page.Absent, Detail: page.Text("not lent")}
		if len(holders) > 0 {
			row.Mark, row.Detail = page.OK, page.Text("lent to "+strings.Join(holders, ", "))
		}
		items = append(items, page.Item{Row: &row})
	}
	return items
}

func keptItems(kept []string) []page.Item {
	if len(kept) == 0 {
		return []page.Item{{Empty: &page.Empty{Noun: "kept secrets"}}}
	}
	items := make([]page.Item, 0, len(kept))
	for _, account := range kept {
		row := page.Row{ID: account, Label: account, Mark: page.OK, Detail: page.Text("never leaves this host")}
		items = append(items, page.Item{Row: &row})
	}
	return items
}

func Change(verb string, names []string, recall secrets.RecallReport, here page.Context) page.Page {
	text := "off the lend list"
	if verb == "allow" {
		text = "on the lend list"
	}
	rows := make([]page.Row, 0, len(names))
	for _, name := range names {
		rows = append(rows, pages.Done(name, catalog.StateOK, text))
	}
	rows = append(rows, recallRows(recall)...)
	return pages.Outcome("secrets "+verb, here, nil, rows, "secrets to "+verb)
}

func Lent(r secrets.LendReport, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(r.Names))
	for _, name := range r.Names {
		rows = append(rows, pages.Done(name, catalog.StateOK, "lent to "+r.To))
	}
	p := pages.Outcome("secrets lend", here, nil, rows, "secrets to lend")
	p.Hints = []page.Hint{*page.Cmd("station secrets recall --from " + r.To)}
	return p
}

func Recalled(r secrets.RecallReport, here page.Context) page.Page {
	return pages.Outcome("secrets recall", here, nil, recallRows(r), "secrets to recall")
}

func recallRows(r secrets.RecallReport) []page.Row {
	var rows []page.Row
	vms := make([]string, 0, len(r.From))
	for vm := range r.From {
		vms = append(vms, vm)
	}
	sort.Strings(vms)
	for _, vm := range vms {
		for _, name := range r.From[vm] {
			row := pages.Done(name+"@"+vm, catalog.StateOK, "recalled from "+vm)
			row.Label = name
			rows = append(rows, row)
		}
	}
	for i, w := range r.Warnings {
		row := pages.Done("recall/"+strconv.Itoa(i), catalog.StateDegraded, w)
		row.Label = "recall"
		rows = append(rows, row)
	}
	return rows
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
