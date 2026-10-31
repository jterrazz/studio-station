package config

import (
	"strings"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/present"
)

func Detail(r config.Report) present.Document {
	lines := []present.Cell{{Text: r.Item.Description}}
	if r.Item.Help != "" {
		lines = append(lines, present.Cell{Text: r.Item.Help, Tone: present.Muted})
	}
	var facts []string
	if r.Item.Requires != "" {
		facts = append(facts, "requires "+r.Item.Requires)
	}
	if len(r.Item.Inputs) > 0 {
		names := make([]string, 0, len(r.Item.Inputs))
		for _, in := range r.Item.Inputs {
			names = append(names, in.Label)
		}
		facts = append(facts, "asks "+strings.Join(names, ", "))
	}
	if r.Item.NeedsRoot {
		facts = append(facts, "needs root")
	}
	if r.Item.Reversible {
		facts = append(facts, "reversible")
	} else {
		facts = append(facts, "not reversible")
	}
	lines = append(lines, present.Cell{Text: strings.Join(facts, " · "), Tone: present.Muted})
	if d := detail(r); d != "" {
		lines = append(lines, present.Cell{Text: d, Tone: present.Muted})
	}
	return present.Document{Command: "station config", Blocks: []present.Block{present.Lines{Lines: lines}}}
}
