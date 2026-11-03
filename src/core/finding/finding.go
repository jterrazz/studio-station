package finding

import (
	"slices"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

type Severity string

const (
	Broken     Severity = "broken"
	NeedsALook Severity = "needs-a-look"
)

type Fix struct {
	// invariant: a fix that needs admin carries a Path or a Command, never a Call.
	Call    string
	Args    map[string]string
	Command string
	Path    string
}

type Finding struct {
	Machine  machine.Ref
	Section  section.ID
	Severity Severity
	Subject  string
	Text     string
	Fix      Fix
}

func Rank(fs []Finding) []Finding {
	// invariant: SPEC §5.1.1. Broken before needs a look; then the one section
	// order; then the Macs in the order they first appear (This Mac first, as
	// every read lists it), each Mac before its VMs. Ties keep their order.
	macs := map[string]int{}
	for _, f := range fs {
		if _, seen := macs[f.Machine.Mac()]; !seen {
			macs[f.Machine.Mac()] = len(macs)
		}
	}
	order := section.Order()
	sectionAt := func(id section.ID) int {
		if i := slices.Index(order, id); i >= 0 {
			return i
		}
		return len(order)
	}
	severityAt := func(s Severity) int {
		if s == Broken {
			return 0
		}
		return 1
	}
	vmAt := func(r machine.Ref) int {
		if r.On == "" {
			return 0
		}
		return 1
	}
	out := slices.Clone(fs)
	slices.SortStableFunc(out, func(a, b Finding) int {
		for _, d := range []int{
			severityAt(a.Severity) - severityAt(b.Severity),
			sectionAt(a.Section) - sectionAt(b.Section),
			macs[a.Machine.Mac()] - macs[b.Machine.Mac()],
			vmAt(a.Machine) - vmAt(b.Machine),
		} {
			if d != 0 {
				return d
			}
		}
		return 0
	})
	return out
}
