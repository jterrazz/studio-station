package machines

import "github.com/jterrazz/studio-station/src/present"

func Change(verb, name, change, context string) present.Answer {
	return present.Answer{Doc: present.Document{
		Command: "station machines " + verb,
		Context: context,
		Blocks: []present.Block{present.Table{Rows: []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: name, Tone: present.Strong},
			{Text: change, Tone: present.Muted},
		}}}}},
	}}
}
