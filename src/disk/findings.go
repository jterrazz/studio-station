package disk

import (
	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Findings(r Reading, m machine.Ref) []finding.Finding {
	// invariant: a misplaced folder needs a look, fixed by moving it.
	var out []finding.Finding
	for _, row := range r.Of(Misplaced) {
		if len(row.Paths) == 0 {
			continue
		}
		path := row.Paths[0]
		out = append(out, finding.Finding{Machine: m, Section: section.Disk, Severity: finding.NeedsALook, Subject: path,
			Text: path + " is misplaced", Fix: finding.Fix{Call: "disk.move", Args: map[string]string{"path": path}}})
	}
	return out
}
