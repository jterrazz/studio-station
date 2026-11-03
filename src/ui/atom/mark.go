package atom

import (
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Glyph(m page.Mark) string {
	switch m {
	case page.OK:
		return token.MarkOK
	case page.Attention:
		return token.MarkAttention
	case page.Absent:
		return token.MarkAbsent
	case page.Broken:
		return token.MarkBroken
	case page.Awaiting:
		return token.MarkPending
	case page.Unjudged:
	}
	return ""
}

func Tone(st token.Styles, m page.Mark) lipgloss.Style {
	switch m {
	case page.OK:
		return st.Good
	case page.Attention:
		return st.Warn
	case page.Broken:
		return st.Bad
	case page.Absent, page.Awaiting, page.Unjudged:
	}
	return st.Muted
}

func Mark(st token.Styles, m page.Mark) string {
	g := Glyph(m)
	if g == "" {
		return Pad("", token.MarkWidth)
	}
	return Tone(st, m).Render(g) + " "
}

func Key(st token.Styles, key, label string) string {
	return st.Accent.Render(KeyName(key)) + " " + st.Muted.Render(label)
}

func KeyName(key string) string {
	switch key {
	case "up/down":
		return token.KeyUp + token.KeyDown
	case "up":
		return token.KeyUp
	case "down":
		return token.KeyDown
	}
	return key
}

func Count(t page.Tally) string { return strconv.Itoa(t.N) + " " + Glyph(t.Mark) }
