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

	Arrow    = "→"
	Sep      = " · "
	Ellipsis = "…"
	Rule     = "─"
	Divider  = "│"
	TreeMid  = "├─ "
	TreeEnd  = "└─ "
	TreeBar  = "│  "
	FoldOpen = "-"
	FoldShut = "+"
	Spinner  = "⣾⣽⣻⢿⡿⣟⣯⣷"
	Spark    = "▁▂▃▄▅▆▇█"
	GaugeOn  = "█"
	GaugeOff = "░"
	QRUpper  = "▀"
	QRLower  = "▄"
	QRFull   = "█"
	KeyUp    = "↑"
	KeyDown  = "↓"
	Charging = "⚡"
	Power    = "⏻"
	Unknown  = "?"
	Corner   = "┌┐└┘"
	Cursor   = "▸"
	Fleet    = "◇"
	Crumb    = " › "
	SideMid  = "├ "
	SideEnd  = "└ "
)

func Glyphs() []string {
	return []string{
		MarkOK, MarkAttention, MarkAbsent, MarkBroken, MarkPending, MarkOffHere, MarkNotFor, MarkUnknown,
		Arrow, "·", Ellipsis, Rule, Divider, "├", "└", Spinner, Spark, GaugeOff, QRUpper, QRLower,
		KeyUp, KeyDown, Charging, Power, Corner, Cursor, Fleet, "›",
	}
}
