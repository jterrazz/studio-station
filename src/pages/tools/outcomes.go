package tools

import (
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func Outcomes(verb string, outcomes []tools.Outcome, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(outcomes))
	for _, o := range outcomes {
		rows = append(rows, pages.Done(o.Name, o.Result, o.Detail))
		for _, s := range o.Steps {
			step := pages.Done(o.Name+"/"+s.Item, s.Result, s.Detail)
			step.Label = s.Item
			rows = append(rows, step)
		}
	}
	return pages.Outcome("tools "+verb, here, nil, rows, "tools to "+verb)
}
