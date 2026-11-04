package apps

import (
	"strings"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/page"
)

func Show(r apps.Report, lives bool, here page.Context) page.Page {
	// invariant: an app's detail (SPEC §5.3): its state, method, version,
	// where it lives, its category, what it brings and what it keeps on disk.
	t := r.Tool
	row := Row(r)
	if !lives {
		row = page.Row{ID: t.Name, Label: t.Name, Mark: page.NotForThisMachine, Detail: page.Text("not for this machine")}
	}
	fact := func(key string, v page.Value) page.Item { return page.Item{Fact: &page.Fact{Key: key, Value: v}} }
	facts := []page.Item{
		fact("method", method(r)),
		fact("version", page.Version(r.Version)),
		fact("lives on", page.Text(apps.LivesOn(t.Places()))),
		fact("category", page.Text(string(t.Category))),
	}
	if t.Description != "" {
		facts = append(facts, fact("what", page.Text(t.Description)))
	}
	if len(t.Then) > 0 {
		facts = append(facts, fact("brings", page.Text(strings.Join(t.Then, ", "))))
	}
	for _, s := range t.Storage.Paths {
		facts = append(facts, fact("keeps", page.Text(s.Path)))
	}
	p := page.Page{
		ID:      "app",
		Command: "apps " + t.Name,
		Context: here,
		Sections: []page.Section{
			{ID: "app", Title: t.Name, Role: page.Banner, Columns: Columns(), Items: []page.Item{{Row: &row}}},
			{ID: "detail", Title: "Detail", Fold: page.Open, Uncounted: true, Items: facts},
		},
	}
	return page.Seal(p)
}
