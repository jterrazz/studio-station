package setup

import (
	"github.com/jterrazz/studio-station/src/domain/setup"
	"github.com/jterrazz/studio-station/src/present"
)

func Outcome(report setup.Report, context string) present.Answer {
	rows := make([]present.Row, 0, len(report.Steps))
	for _, step := range report.Steps {
		cells := []present.Cell{{Text: step.Name, Tone: present.Strong}, {Text: step.Detail, Tone: present.Muted}}
		if step.Log != "" {
			cells = append(cells, present.Cell{Text: step.Log, Tone: present.Muted})
		}
		rows = append(rows, present.Row{Mark: present.MarkOf(step.State), Cells: cells})
	}
	return present.Answer{Doc: present.Document{
		Command: "station setup",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
