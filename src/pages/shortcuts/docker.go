package shortcuts

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func Docker(o shortcuts.Outcome, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(o.Removals))
	for _, r := range o.Removals {
		detail := page.Text("none to remove")
		if r.Count > 0 {
			detail = page.Count(int64(r.Count), "removed")
		}
		row := pages.Done(r.What, catalog.StateOK, "")
		row.Detail = detail
		rows = append(rows, row)
	}
	return pages.Outcome("run docker "+o.Shortcut, here, nil, rows, "steps")
}
