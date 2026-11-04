package config

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const (
	KeyInstall   = "i"
	KeyUninstall = "u"
)

type ItemJSON struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Places   []string `json:"places"`
	State    string   `json:"state"`
	Detail   string   `json:"detail"`
}

type ItemsJSON struct {
	Schema string     `json:"schema"`
	Items  []ItemJSON `json:"items"`
}

func ItemsContract(reports []config.Report) ItemsJSON {
	body := ItemsJSON{Schema: "station.config/v1", Items: make([]ItemJSON, 0, len(reports))}
	for _, r := range reports {
		body.Items = append(body.Items, ItemObject(r))
	}
	return body
}

func ItemObject(r config.Report) ItemJSON {
	places := make([]string, len(r.Item.Places))
	for i, p := range r.Item.Places {
		places[i] = string(p)
	}
	return ItemJSON{Name: r.Item.Name, Category: string(r.Item.Category), Places: places, State: string(r.State), Detail: r.Detail}
}

func categories() []catalog.ConfigCategory {
	return []catalog.ConfigCategory{
		catalog.CategoryShell, catalog.CategoryTerminal, catalog.CategorySecurity, catalog.CategoryEditor,
		catalog.CategorySystem, catalog.CategoryHeadless, catalog.CategoryNetwork,
	}
}

func Items(reports []config.Report, here page.Context) page.Page {
	p := page.Page{
		ID:      "config",
		Command: "config",
		Context: here,
		Hints:   []page.Hint{*page.Cmd("station config install <item...>"), *page.Cmd("station config uninstall <item...>")},
		Actions: Actions(reports),
	}
	for _, category := range categories() {
		var items []page.Item
		for _, r := range reports {
			if r.Item.Category == category {
				row := Row(r)
				items = append(items, page.Item{Row: &row})
			}
		}
		if len(items) == 0 {
			continue
		}
		p.Sections = append(p.Sections, page.Section{
			ID:         strings.ToLower(string(category)),
			Title:      string(category),
			Fold:       page.Auto,
			Columns:    Columns(),
			Items:      items,
			PipeCounts: true,
		})
	}
	if len(p.Sections) == 0 {
		p.Sections = []page.Section{{ID: "config", Title: "Config", Role: page.Banner, Columns: Columns(),
			Items: []page.Item{{Empty: &page.Empty{Noun: "items", Hint: page.Cmd("station setup")}}}}}
	}
	return page.Seal(p)
}

func Columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "description", Kind: page.KindText, Keep: 20},
	}
}

func Actions(reports []config.Report) []page.Action {
	var reversible []string
	for _, r := range reports {
		if r.Item.Reversible {
			reversible = append(reversible, r.Item.Name)
		}
	}
	actions := []page.Action{{Key: KeyInstall, Label: "install", On: page.OnRow, Priority: 1, Mutates: true}}
	if len(reversible) > 0 {
		actions = append(actions, page.Action{Key: KeyUninstall, Label: "uninstall", On: page.OnRow, Rows: reversible, Priority: 2, Mutates: true, Confirm: &page.Confirm{Yes: "uninstall"}})
	}
	return actions
}

func UninstallConfirm(name string) page.Confirm {
	return page.Confirm{Question: "Uninstall " + name + "?", Yes: "uninstall"}
}

const differs = "differs from station's"

func Row(r config.Report) page.Row {
	name := r.Item.Name
	row := page.Row{ID: name, Label: name, Mark: pages.MarkOf(r.State), Cells: []page.Value{page.Text(r.Item.Description)}}
	install := page.KeyHint("station config install "+name, KeyInstall)
	detail := r.Detail
	switch {
	case r.State == catalog.StatePending:
		row.Detail = page.Pending()
		return row
	case r.State == catalog.StateAbsent && detail == "" && !r.Applies:
		row.Reason, detail, row.Hint = page.Missing, "needs "+r.Item.Requires, page.Cmd("station apps install "+r.Item.Requires)
	case r.State == catalog.StateAbsent:
		row.Hint = install
	case strings.HasPrefix(detail, differs):
		row.Reason, detail, row.Hint = page.Differs, differs, install
	case r.State == catalog.StateDegraded:
		row.Reason, row.Hint = page.Partial, install
	case r.State == catalog.StateUnknown:
		row.Reason = page.Late
	case r.State == catalog.StateBroken:
		row.Reason = page.Down
	case r.State == catalog.StateOK:
	}
	row.Detail = page.Text(detail)
	if (row.Mark == page.Attention || row.Mark == page.Broken) && row.Detail.IsNone() && row.Hint == nil {
		row.Detail = page.Text(string(r.State))
	}
	return row
}

func InputsForm(name string, inputs []catalog.Input, defaults map[string]string) page.Form {
	f := page.Form{Title: "Configure " + name}
	for _, in := range inputs {
		field := page.Field{Key: in.Name, Label: in.Label, Note: in.Help, Default: defaults[in.Name], Required: in.Required, Kind: page.FieldText}
		switch in.Kind {
		case catalog.InputPassword:
			field.Kind = page.FieldSecret
		case catalog.InputSelect:
			field.Kind = page.FieldSelect
			for _, o := range in.Options {
				field.Options = append(field.Options, page.Option{Value: o})
			}
		case catalog.InputConfirm:
			field.Kind = page.FieldConfirm
		case catalog.InputText:
		}
		f.Fields = append(f.Fields, field)
	}
	return f
}

func Changes(verb string, changes []config.Change, here page.Context) page.Page {
	var rows []page.Row
	extra := make([]page.Section, 0, 2*len(changes))
	for _, c := range changes {
		head := pages.Done(c.Item, c.Result, "")
		if len(c.Steps) == 1 {
			head.Detail = page.Text(stepText(c.Steps[0]))
			rows = append(rows, head)
		} else {
			rows = append(rows, head)
			for i, s := range c.Steps {
				step := pages.Done(c.Item+"/"+strconv.Itoa(i), s.State, s.Detail)
				step.Label = s.Label
				rows = append(rows, step)
			}
		}
		extra = append(extra, facts(c.Item, "before", c.Before)...)
		extra = append(extra, facts(c.Item, "after", c.After)...)
	}
	return pages.Outcome("config "+verb, here, nil, rows, "items to "+verb, extra...)
}

func stepText(s config.Step) string {
	if s.Detail == "" {
		return s.Label
	}
	return s.Label + ": " + s.Detail
}

func facts(item, when string, list []config.Fact) []page.Section {
	if len(list) == 0 {
		return nil
	}
	items := make([]page.Item, 0, len(list))
	for _, f := range list {
		items = append(items, page.Item{Fact: &page.Fact{Key: f.Key, Value: page.Text(f.Value)}})
	}
	return []page.Section{{ID: item + "-" + when, Title: item + " " + when, Columns: pages.OutcomeColumns(), Items: items}}
}
