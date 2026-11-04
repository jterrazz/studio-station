package settings

import (
	"slices"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

type Group string

const (
	SetByStation Group = "set"
	// value: the opt-in group of Set by station for a Mac that hosts
	// servers: applied by name only, folded until one of it is applied.
	AlwaysOn Group = "always-on"
)

type State string

const (
	StateApplied           State = "applied"
	StateChangedSince      State = "changed-since"
	StateNotApplied        State = "not-applied"
	StateOffHere           State = "off-here"
	StateNotForThisMachine State = "not-for-this-machine"
	// value: the row is still being read.
	StateReading State = "reading"
)

type Observation struct {
	// value: what the setting reads now on this machine, as the overview
	// reads it: in place, missing, partly there, unknown.
	State  catalog.State
	Detail string
	// value: a fingerprint of what is there now; empty for a setting whose
	// effect has no content to compare (a login, a procedure's yes or no).
	Digest string
	// value: where it lives: "~/.config/starship.toml".
	Where string
	// value: the unified diff of what station would write against what is
	// there, for a setting that writes files; empty when they are the same.
	Diff string
	// value: a setting that writes no file: its values now, and what station
	// would set.
	Before, After []KeyValue
	// value: an effect station cannot read back once done (the Dock reset).
	Blind bool
}

func (o Observation) Present() bool { return o.State == catalog.StateOK }

type Row struct {
	Item  catalog.ConfigItem
	Group Group
	State State
	// value: ChangedSince and Applied: when station applied it.
	Since time.Time
	Where string
	// value: NotForThisMachine: the reason, "Mac only"; NotApplied: what it
	// waits for, "needs starship".
	Why string
	// value: the app it needs that is not installed here.
	Needs string
}

func GroupOf(it catalog.ConfigItem) Group {
	if it.Category == catalog.CategoryHeadless {
		return AlwaysOn
	}
	return SetByStation
}

func PlaceReason(places []catalog.MachineKind) string {
	// value: "not for this machine" with the kinds it lives on, in the
	// owner's words.
	mac := slices.Contains(places, catalog.Host)
	vm := slices.Contains(places, catalog.Linux) || slices.Contains(places, catalog.MacOS)
	switch {
	case mac && !vm:
		return "Mac only"
	case !mac && vm:
		return "VMs only"
	}
	return "Macs and macOS VMs only"
}

// invariant: one setting's state on one machine, from its place, the owner's
// Off here, what station recorded when it applied it and what is there now.
// Off here wins over everything but the place; a record whose fingerprint no
// longer matches, or whose effect is gone, is Changed since; with no record,
// a setting already exactly as station writes it is Applied, from before
// station kept records.
func JudgeSet(it catalog.ConfigItem, lives bool, applied *Applied, now Observation, off bool) Row {
	row := Row{Item: it, Group: GroupOf(it), Where: now.Where}
	switch {
	case !lives:
		row.State, row.Where, row.Why = StateNotForThisMachine, "", PlaceReason(it.Places)
	case off:
		row.State = StateOffHere
	case now.State == catalog.StatePending:
		row.State = StateReading
	case applied != nil && now.Blind:
		row.State, row.Since = StateApplied, applied.At
	case applied != nil && (!now.Present() || applied.Digest != now.Digest):
		row.State, row.Since = StateChangedSince, applied.At
	case applied != nil:
		row.State, row.Since = StateApplied, applied.At
	case now.Present() && now.Diff == "" && !now.Blind:
		row.State = StateApplied
	default:
		row.State = StateNotApplied
	}
	return row
}
