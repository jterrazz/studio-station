package present

import "github.com/jterrazz/studio-station/src/catalog"

type Mark int

const (
	NoMark Mark = iota
	MarkOK
	MarkDegraded
	MarkAbsent
	MarkBroken
	// value: a value whose probe has not answered; a tty and the TUI only, since
	// pipe and JSON wait for every probe.
	MarkPending
)

func (m Mark) Glyph() string {
	switch m {
	case MarkOK:
		return "●"
	case MarkDegraded:
		return "◐"
	case MarkAbsent:
		return "○"
	case MarkBroken:
		return "✗"
	case MarkPending:
		return "◌"
	case NoMark:
		return ""
	}
	return ""
}

func MarkOf(s catalog.State) Mark {
	switch s {
	case catalog.StateOK:
		return MarkOK
	case catalog.StateDegraded, catalog.StateUnknown:
		return MarkDegraded
	case catalog.StateAbsent:
		return MarkAbsent
	case catalog.StateBroken:
		return MarkBroken
	case catalog.StatePending:
		return MarkPending
	}
	return NoMark
}
