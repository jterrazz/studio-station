package shortcuts

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	"github.com/jterrazz/studio-station/src/present"
)

func Docker(o shortcuts.Outcome, context string) present.Answer {
	rows := make([]present.Row, 0, len(o.Removals))
	for _, r := range o.Removals {
		detail := "none to remove"
		if r.Count > 0 {
			detail = strconv.Itoa(r.Count) + " removed"
		}
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: r.What, Tone: present.Strong},
			{Text: detail, Tone: present.Muted},
		}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station run docker " + o.Shortcut,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
