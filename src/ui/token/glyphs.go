package token

const (
	MarkOK        = "●"
	MarkAttention = "◐"
	MarkAbsent    = "○"
	MarkBroken    = "✗"
	MarkPending   = "◌"
	MarkOffHere   = "⊘"
	MarkNotFor    = "–"
	MarkUnknown   = "?"

	Arrow       = "→"
	Sep         = " · "
	Ellipsis    = "…"
	Rule        = "─"
	Divider     = "│"
	TreeMid     = "├─ "
	TreeEnd     = "└─ "
	TreeBar     = "│  "
	FoldOpen    = "-"
	FoldShut    = "+"
	Spinner     = "⣾⣽⣻⢿⡿⣟⣯⣷"
	Spark       = "▁▂▃▄▅▆▇█"
	GaugeOn     = "█"
	GaugeOff    = "░"
	QRUpper     = "▀"
	QRLower     = "▄"
	QRFull      = "█"
	KeyUp       = "↑"
	KeyDown     = "↓"
	KeyLeft     = "←"
	KeyRight    = Arrow
	Charging    = "⚡"
	Power       = "⏻"
	Unknown     = "?"
	Corner      = "┌┐└┘"
	Cursor      = "▸"
	AllMachines = "◇"
	Crumb       = " › "
	SideMid     = "├ "
	SideEnd     = "└ "
	// value: a row ticked for a group action, and a toast's success.
	Tick = "✓"
	// value: what leads a text the owner types (a dialog's field).
	Typed = "›"
)

func Glyphs() []string {
	return []string{
		MarkOK, MarkAttention, MarkAbsent, MarkBroken, MarkPending, MarkOffHere, MarkNotFor, MarkUnknown,
		Arrow, "·", Ellipsis, Rule, Divider, "├", "└", Spinner, Spark, GaugeOff, QRUpper, QRLower,
		KeyUp, KeyDown, KeyLeft, Charging, Power, Corner, Cursor, AllMachines, Typed, Tick,
	}
}
