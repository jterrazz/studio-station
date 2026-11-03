package token

const (
	MarkOK        = "●"
	MarkAttention = "◐"
	MarkAbsent    = "○"
	MarkBroken    = "✗"
	MarkPending   = "◌"

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
	Unknown  = "?"
	Corner   = "┌┐└┘"
)

func Glyphs() []string {
	return []string{
		MarkOK, MarkAttention, MarkAbsent, MarkBroken, MarkPending,
		Arrow, "·", Ellipsis, Rule, Divider, "├", "└", Spinner, Spark, GaugeOff, QRUpper, QRLower,
		KeyUp, KeyDown, Charging, Corner,
	}
}
