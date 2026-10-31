package config

import (
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/present"
)

func Changes(verb string, changes []config.Change, context string) present.Answer {
	// value: a change with one step reads as one row (the item and what happened);
	// several steps follow the item as their own rows, and the facts it read before
	// and after follow as two key-value blocks.
	var rows []present.Row
	for _, c := range changes {
		rows = append(rows, itemRows(c)...)
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Cells: []present.Cell{{Text: "nothing to " + verb, Tone: present.Muted}}})
	}
	blocks := make([]present.Block, 0, 1+2*len(changes))
	blocks = append(blocks, present.Table{Rows: rows})
	for _, c := range changes {
		blocks = append(blocks, facts(c.Item+" before", c.Before)...)
		blocks = append(blocks, facts(c.Item+" after", c.After)...)
	}
	return present.Answer{Doc: present.Document{Command: "station config " + verb, Context: context, Blocks: blocks}}
}

func itemRows(c config.Change) []present.Row {
	mark := present.MarkOf(c.Result)
	if len(c.Steps) == 1 {
		s := c.Steps[0]
		return []present.Row{{Mark: mark, Cells: []present.Cell{{Text: c.Item, Tone: present.Strong}, {Text: stepText(s), Tone: present.Muted}}}}
	}
	rows := []present.Row{{Mark: mark, Cells: []present.Cell{{Text: c.Item, Tone: present.Strong}}}}
	for _, s := range c.Steps {
		rows = append(rows, present.Row{Mark: present.MarkOf(s.State), Cells: []present.Cell{{Text: s.Label}, {Text: s.Detail, Tone: present.Muted}}})
	}
	return rows
}

func stepText(s config.Step) string {
	if s.Detail == "" {
		return s.Label
	}
	return s.Label + ": " + s.Detail
}

func facts(title string, list []config.Fact) []present.Block {
	if len(list) == 0 {
		return nil
	}
	pairs := make([]present.Pair, len(list))
	for i, f := range list {
		pairs[i] = present.Pair{Key: f.Key, Value: present.Cell{Text: f.Value, Tone: present.Muted}}
	}
	return []present.Block{present.Section{Title: title, Blocks: []present.Block{present.KeyValues{Pairs: pairs}}}}
}
