package clean

import (
	"strings"

	"github.com/jterrazz/studio-station/src/domain/clean"
	"github.com/jterrazz/studio-station/src/present"
)

func Detail(r clean.Report) present.Document {
	lines := []present.Cell{{Text: r.Item.Description}}
	if r.Available {
		lines = append(lines, present.Cell{Text: "reclaims " + present.Bytes(r.Bytes)})
	} else {
		lines = append(lines, present.Cell{Text: "not available here", Tone: present.Muted})
	}
	if r.Item.Requires != "" {
		lines = append(lines, present.Cell{Text: "requires " + r.Item.Requires, Tone: present.Muted})
	}
	if len(r.Item.Dirs) > 0 {
		lines = append(lines, present.Cell{Text: strings.Join(r.Item.Dirs, ", "), Tone: present.Muted})
	}
	return present.Document{Command: "station clean", Blocks: []present.Block{present.Lines{Lines: lines}}}
}
