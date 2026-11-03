package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Tab struct {
	Label string
	// value: the first tab of the fleet half: the header draws the divider before it.
	Fleet bool
}

type Chrome struct {
	styles token.Styles
	spin   spinner.Model
}

func NewChrome() Chrome {
	st := token.New(true)
	return Chrome{styles: st, spin: atom.Spinner(st)}
}

func (c Chrome) Themed(dark bool) Chrome {
	c.styles = token.New(dark)
	c.spin.Style = atom.Spinner(c.styles).Style
	return c
}

func (c Chrome) Tick() tea.Msg { return c.spin.Tick() }

func (c Chrome) Turn(msg tea.Msg) (Chrome, tea.Cmd, bool) {
	t, isTick := msg.(spinner.TickMsg)
	if !isTick || t.ID != c.spin.ID() {
		return c, nil, false
	}
	var cmd tea.Cmd
	c.spin, cmd = c.spin.Update(msg)
	return c, cmd, true
}

func (c Chrome) tabLine(tabs []Tab, active, numbered int, loading bool) string {
	st := c.styles
	var b strings.Builder
	b.WriteString(atom.Spaces(token.Margin) + st.Title.Render("station") + atom.Spaces(token.Gap))
	left := numbered
	shown := make([]string, len(tabs))
	for i := len(tabs) - 1; i >= 0; i-- {
		shown[i] = tabs[i].Label
		if i != active && left > 0 {
			shown[i] = strconv.Itoa(i + 1)
			left--
		}
	}
	for i, t := range tabs {
		if i > 0 {
			sep := token.Sep
			if t.Fleet && !tabs[i-1].Fleet {
				sep = " " + token.Divider + " "
			}
			b.WriteString(st.Subtle.Render(sep))
		}
		if i == active {
			b.WriteString(st.Title.Render(shown[i]))
		} else {
			b.WriteString(st.Muted.Render(shown[i]))
		}
	}
	if loading {
		b.WriteString(atom.Spaces(token.Gap) + c.spin.View())
	}
	return b.String()
}

func (c Chrome) Header(tabs []Tab, active int, context page.Context, loading bool, width int) string {
	// invariant: the tab line never wraps (UI §7.7): the context shortens to
	// the machine name, then to the kind; the inactive tabs turn into their
	// number key one at a time from the last backwards; then the context goes.
	short := context.Kind
	if short == "" {
		short = "host"
	}
	shortened := []string{short}
	if context.Registered && context.Machine != "" {
		shortened = []string{context.Machine, short}
	}
	left, ctx := "", ""
	for n := range tabs {
		left = c.tabLine(tabs, active, n, loading)
		candidates := shortened
		if n == 0 && width > token.NarrowWidth {
			candidates = append([]string{organism.Context(context)}, shortened...)
		}
		for _, x := range candidates {
			if atom.Width(left)+token.Gap+atom.Width(x)+1 <= width {
				ctx = x
				break
			}
		}
		if ctx != "" {
			break
		}
	}
	gap := max(width-1-atom.Width(left)-atom.Width(ctx), 1)
	return left + atom.Spaces(gap) + c.styles.Muted.Render(ctx) + "\n" + organism.Rule(c.styles, width)
}

func (c Chrome) StatusBar(m page.Mark, text, hint string, width int) string {
	return organism.StatusBar(c.styles, m, text, hint, width)
}

func (c Chrome) Screen(header string, body func(room int) string, bar, footer string, height int) string {
	room := max(height-strings.Count(header, "\n")-1-strings.Count(footer, "\n")-1-1, 1)
	lines := strings.Split(body(room), "\n")
	if len(lines) > room {
		lines = lines[:room]
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	return header + "\n" + strings.Join(lines, "\n") + "\n" + bar + "\n" + footer
}
