package tools

import (
	"strings"

	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
)

func Detail(r tools.Report) present.Document {
	var facts []string
	if len(r.Tool.Provides) > 0 {
		facts = append(facts, "provides "+strings.Join(r.Tool.Provides, ", "))
	}
	if len(r.Tool.Requires) > 0 {
		facts = append(facts, "requires "+strings.Join(r.Tool.Requires, ", "))
	}
	if r.Managed {
		facts = append(facts, "managed by "+string(r.Method))
	} else if r.Method != "" {
		facts = append(facts, "installed with "+string(r.Method))
	}
	lines := []present.Cell{{Text: r.Tool.Description}}
	if len(facts) > 0 {
		lines = append(lines, present.Cell{Text: strings.Join(facts, " · "), Tone: present.Muted})
	}
	if note := ToolObject(r).Detail; note != "" {
		lines = append(lines, present.Cell{Text: note, Tone: present.Muted})
	}
	return present.Document{Command: "station tools", Blocks: []present.Block{present.Lines{Lines: lines}}}
}
