package apps

import "github.com/jterrazz/studio-station/src/catalog"

type State string

const (
	UpdateAvailable State = "update-available"
	Failed          State = "failed"
	NotInstalled    State = "not-installed"
	NotManaged      State = "not-managed"
	OK              State = "ok"
	NotKnown        State = "not-known"
	Pending         State = "pending"
)

func Groups() []State {
	// value: the Apps groups in their one order (SPEC §5.3), what needs the
	// owner first.
	return []State{UpdateAvailable, Failed, NotInstalled, NotManaged, NotKnown, OK}
}

func StateOf(r Report) State {
	// invariant: an app is OK only when its catalogue method holds it and
	// nothing newer waits; present any other way it is Not managed.
	if r.Pending {
		return Pending
	}
	switch r.State {
	case catalog.StateOK:
		if !r.Managed {
			return NotManaged
		}
		return OK
	case catalog.StateDegraded:
		if r.Why == WhyOutdated {
			return UpdateAvailable
		}
		return NotManaged
	case catalog.StateBroken:
		return Failed
	case catalog.StateAbsent:
		return NotInstalled
	case catalog.StateUnknown, catalog.StatePending:
	}
	return NotKnown
}

func (s State) Installed() bool {
	return s == OK || s == UpdateAvailable || s == NotManaged || s == Failed
}

func Counts(reports []Report) map[State]int {
	out := map[State]int{}
	for _, r := range reports {
		out[StateOf(r)]++
	}
	return out
}

func FailedOf(outcomes []Outcome) (n int, first string) {
	// value: how many outcomes failed, and the first of them, whose repair
	// the error names.
	for _, o := range outcomes {
		if o.Result == catalog.StateBroken {
			if n == 0 {
				first = o.Name
			}
			n++
		}
	}
	return n, first
}

func Elsewhere(reports []Report) []catalog.Tool {
	// value: the catalogue apps that live on other machines than this one:
	// every app the list does not place here.
	here := map[string]bool{}
	for _, r := range reports {
		here[r.Tool.Name] = true
	}
	var out []catalog.Tool
	for _, t := range catalog.Apps {
		if !here[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
