package setup

import (
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func Outcome(report setup.Report, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(report.Steps))
	for _, step := range report.Steps {
		r := pages.Done(step.Name, step.State, step.Detail)
		r.Cells = []page.Value{page.Text(step.Detail)}
		r.Detail = page.Text(step.Log)
		rows = append(rows, r)
	}
	return pages.Outcome("setup", here, Columns(), rows, "steps")
}

func Columns() []page.Column {
	return []page.Column{{Key: "step", Kind: page.KindText, Keep: 100}, {Key: "detail", Kind: page.KindText, Keep: 50}}
}

func NameForm(suggestion string) page.Form {
	return page.Form{Fields: []page.Field{{
		Key: "name", Label: "Name this machine", Kind: page.FieldText, Default: suggestion,
		Note: "lowercase letters, digits and -, as in macbook or mac-mini",
	}}}
}
