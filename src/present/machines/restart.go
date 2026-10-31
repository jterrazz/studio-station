package machines

import (
	"time"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/present"
)

func Restart(events []machines.RestartEvent, r machines.RestartReport, context string) present.Answer {
	rows := make([]present.Row, 0, len(events)+1)
	for _, e := range events {
		rows = append(rows, present.Row{Cells: []present.Cell{
			{Text: e.Phase, Tone: present.Muted},
			{Text: e.Detail, Tone: present.Muted},
		}})
	}
	rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
		{Text: r.Name, Tone: present.Strong},
		{Text: "restarted in " + r.Waited.Round(time.Second).String()},
	}})
	return present.Answer{Doc: present.Document{
		Command: "station machines restart " + r.Name,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
