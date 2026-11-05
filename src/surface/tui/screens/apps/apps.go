package appsscreen

import (
	"slices"

	"github.com/jterrazz/studio-station/src/page"
	pageapps "github.com/jterrazz/studio-station/src/pages/apps"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	// invariant: a gesture takes the rows it means: install the apps not
	// installed, update those with an update waiting, remove the installed.
	name := func(r page.Row) []string { return []string{r.ID} }
	keep := func(marks ...page.Mark) {
		rows = slices.DeleteFunc(slices.Clone(rows), func(r page.Row) bool { return !slices.Contains(marks, r.Mark) })
	}
	switch a.Key {
	case pageapps.KeyInstall:
		keep(page.Absent, page.Broken)
	case pageapps.KeyUpdate:
		keep(page.Attention)
	case pageapps.KeyRemove:
		keep(page.OK, page.Attention)
	}
	if a.On == page.OnRow && len(rows) == 0 {
		return gesture.Gesture{}, false
	}
	switch a.Key {
	case pageapps.KeyInstall:
		return gesture.Each(rows, gesture.V("Install", "Installing", "Installed"), func(r page.Row) []string { return append([]string{"apps", "install"}, name(r)...) }), true
	case pageapps.KeyUpdate:
		return gesture.Each(rows, gesture.V("Update", "Updating", "Updated"), func(r page.Row) []string { return append([]string{"apps", "update"}, name(r)...) }), true
	case pageapps.KeyRemove:
		return gesture.Each(rows, gesture.V("Remove", "Removing", "Removed"), func(r page.Row) []string { return append([]string{"apps", "remove"}, name(r)...) }), true
	case pageapps.KeyUpdateByMethod:
		return gesture.Gesture{Pick: &gesture.Pick{Title: "Update by method", Options: methods(p), Then: func(m string) gesture.Gesture {
			return gesture.Gesture{Steps: []gesture.Step{gesture.V("Update", "Updating", "Updated").Step("every "+m+" app", []string{"apps", "update", "--method", m})}}
		}}}, true
	case pageapps.KeyOpen:
		if len(rows) == 1 {
			return gesture.Gesture{Read: []string{"apps", rows[0].ID}}, true
		}
	}
	return gesture.Gesture{}, false
}

// value: the methods of the apps with an update waiting, in their order.
func methods(p page.Page) []string {
	var out []string
	for _, sec := range p.Sections {
		if sec.ID != "updates" {
			continue
		}
		for _, it := range sec.Items {
			if it.Row == nil {
				continue
			}
			if m := ui.Plain(gesture.Cell(sec, *it.Row, "method")); m != "" && !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}
