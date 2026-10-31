package kit

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/theme"
)

type Tab struct {
	Label string
	Fleet bool
}

func Header(t theme.Theme, tabs []Tab, active int, context string, width int) string {
	var b strings.Builder
	b.WriteString(margin + t.Title.Render("station") + "   ")
	for i, tab := range tabs {
		if i > 0 {
			sep := " · "
			if tab.Fleet && !tabs[i-1].Fleet {
				sep = " │ "
			}
			b.WriteString(t.Subtle.Render(sep))
		}
		if i == active {
			b.WriteString(t.Title.Render(tab.Label))
		} else {
			b.WriteString(t.Subtle.Render(tab.Label))
		}
	}
	left := b.String()
	gap := max(width-1-lipgloss.Width(left)-lipgloss.Width(context), 1)
	line := left + strings.Repeat(" ", gap) + t.Muted.Render(context)
	return line + "\n" + margin + t.Subtle.Render(strings.Repeat("─", max(width-2, 1)))
}

func NewHelp(t theme.Theme) help.Model {
	h := help.New()
	h.ShortSeparator = " · "
	h.Styles = help.Styles{
		Ellipsis:       t.Subtle,
		ShortKey:       t.Muted,
		ShortDesc:      t.Muted,
		ShortSeparator: t.Subtle,
		FullKey:        t.Muted,
		FullDesc:       t.Muted,
		FullSeparator:  t.Subtle,
	}
	return h
}

func Footer(t theme.Theme, h help.Model, screen, global []key.Binding, busy bool, width int) string {
	prefix := margin
	if busy {
		prefix += t.Warn.Render("busy") + t.Subtle.Render(" · ")
	}
	h.SetWidth(max(width-lipgloss.Width(prefix)-1, 10))
	if h.ShowAll {
		return prefix + h.FullHelpView([][]key.Binding{screen, global})
	}
	short := screen
	if len(short) > 5 {
		short = short[:5]
	}
	return prefix + h.ShortHelpView(append(append([]key.Binding{}, short...), Bind("?", "more", "?")))
}
