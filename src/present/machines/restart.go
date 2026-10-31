package machines

import "github.com/jterrazz/studio-station/src/present"

func Restart(name, context string) present.Answer {
	return present.Answer{Doc: present.Document{
		Command: "station machines restart " + name,
		Context: context,
		Blocks: []present.Block{present.Table{Rows: []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: name, Tone: present.Strong},
			{Text: "restarted"},
		}}}}},
	}}
}
