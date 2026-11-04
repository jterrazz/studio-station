package settings

import (
	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func Findings(rows []Row, m machine.Ref) []finding.Finding {
	// invariant: a setting changed since station applied it needs a look,
	// fixed by reading its diff; one never applied is the owner's choice.
	var out []finding.Finding
	for _, r := range rows {
		if r.State == StateChangedSince {
			out = append(out, finding.Finding{Machine: m, Section: section.Settings, Severity: finding.NeedsALook, Subject: r.Name,
				Text: r.Name + " changed since", Fix: finding.Fix{Call: "settings.diff", Args: map[string]string{"name": r.Name}}})
		}
	}
	return out
}
