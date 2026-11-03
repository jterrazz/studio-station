package tools

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
)

const (
	KeyInstall   = "i"
	KeyUninstall = "u"
	KeyUpgrade   = "U"
)

func Catalogue(reports []tools.Report, here page.Context) page.Page {
	p := page.Page{
		ID:      "tools",
		Command: "tools",
		Context: here,
		Hints:   []page.Hint{*page.Cmd("station tools install <tool...>"), *page.Cmd("station tools upgrade")},
		Actions: Actions(),
	}
	for _, category := range catalog.Categories {
		var items []page.Item
		for _, r := range reports {
			if r.Tool.Category == category {
				row := Row(r)
				items = append(items, page.Item{Row: &row})
			}
		}
		if len(items) == 0 {
			continue
		}
		p.Sections = append(p.Sections, page.Section{
			ID:      sectionID(category),
			Title:   string(category),
			Fold:    page.Auto,
			Columns: Columns(),
			Items:   items,
		})
	}
	if len(p.Sections) == 0 {
		p.Sections = []page.Section{{ID: "tools", Title: "Tools", Role: page.Banner, Columns: Columns(), Items: []page.Item{{Empty: &page.Empty{Noun: "tools"}}}}}
	}
	return page.Seal(p)
}

func sectionID(c catalog.Category) string {
	words := strings.FieldsFunc(strings.ToLower(string(c)), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	return strings.Join(words, "-")
}

func Columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "version", Kind: page.KindVersion, Keep: 50},
		{Key: "method", Kind: page.KindList, Keep: 30},
	}
}

func Actions() []page.Action {
	return []page.Action{
		{Key: KeyInstall, Label: "install", On: page.OnRow, Priority: 1, Mutates: true, Confirm: &page.Confirm{Yes: "install"}},
		{Key: KeyUninstall, Label: "uninstall", On: page.OnRow, Priority: 2, Mutates: true, Confirm: &page.Confirm{Yes: "uninstall"}},
		{Key: KeyUpgrade, Label: "upgrade", On: page.OnRow, Priority: 3, Mutates: true},
	}
}

func Confirm(verb, name string) page.Confirm {
	return page.Confirm{Question: strings.ToUpper(verb[:1]) + verb[1:] + " " + name + "?", Yes: verb}
}

func Row(r tools.Report) page.Row {
	name := r.Tool.Name
	row := page.Row{ID: name, Label: name, Cells: []page.Value{page.Version(r.Version), method(r)}}
	if r.Pending {
		row.Mark, row.Cells[0] = page.Awaiting, page.Pending()
		return row
	}
	detail := r.Detail
	switch r.State {
	case catalog.StateOK:
		row.Mark = page.OK
		if detail == "" {
			detail = r.Live
		}
	case catalog.StateAbsent:
		row.Mark = page.Absent
		if r.Why == tools.WhyMissing {
			row.Reason, row.Hint = page.Missing, page.Cmd("station tools install "+r.Needs)
		} else {
			row.Hint = page.KeyHint("station tools install "+name, KeyInstall)
		}
	case catalog.StateDegraded, catalog.StateUnknown:
		row.Mark = page.Attention
		row.Reason = reason(r.Why)
		switch r.Why {
		case tools.WhyMissing:
			row.Hint = page.Cmd("station tools install " + r.Needs)
		case tools.WhyLate:
			row.Cells[0] = page.Unknown()
		case tools.WhyUnmanaged, tools.WhyPartial, tools.WhyFailing, tools.WhyOutdated:
		}
	case catalog.StateBroken:
		row.Mark, row.Reason = page.Broken, page.Failing
	case catalog.StatePending:
		row.Mark = page.Awaiting
	}
	row.Detail = page.Text(detail)
	return row
}

func reason(w tools.Why) page.Reason {
	switch w {
	case tools.WhyMissing:
		return page.Missing
	case tools.WhyOutdated:
		return page.Outdated
	case tools.WhyPartial:
		return page.Partial
	case tools.WhyUnmanaged:
		return page.Unmanaged
	case tools.WhyFailing:
		return page.Failing
	case tools.WhyLate:
	}
	return page.Late
}

func method(r tools.Report) page.Value {
	if r.Tool.System {
		return page.Texts(string(r.Method), "system")
	}
	return page.Texts(string(r.Method))
}
