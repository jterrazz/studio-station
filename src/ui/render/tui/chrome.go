package tui

import (
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
	frames := make([]string, 0, len(token.Spinner))
	for _, r := range token.Spinner {
		frames = append(frames, string(r))
	}
	return Chrome{styles: st, spin: spinner.New(spinner.WithSpinner(spinner.Spinner{Frames: frames, FPS: token.SpinnerFrame}), spinner.WithStyle(st.Accent))}
}

func (c Chrome) Themed(dark bool) Chrome {
	c.styles = token.New(dark)
	c.spin.Style = c.styles.Accent
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

func (c Chrome) Header(tabs []Tab, active int, context page.Context, loading bool, width int) string {
	st := c.styles
	var b strings.Builder
	b.WriteString(atom.Spaces(token.Margin) + st.Title.Render("station") + atom.Spaces(token.Gap))
	for i, t := range tabs {
		if i > 0 {
			sep := token.Sep
			if t.Fleet && !tabs[i-1].Fleet {
				sep = " " + token.Divider + " "
			}
			b.WriteString(st.Subtle.Render(sep))
		}
		if i == active {
			b.WriteString(st.Title.Render(t.Label))
		} else {
			b.WriteString(st.Muted.Render(t.Label))
		}
	}
	if loading {
		b.WriteString(atom.Spaces(token.Gap) + c.spin.View())
	}
	left := b.String()
	ctx := organism.Context(context)
	if width <= token.NarrowWidth {
		ctx = context.Kind
		if ctx == "" {
			ctx = "host"
		}
	}
	gap := max(width-1-atom.Width(left)-atom.Width(ctx), 1)
	return left + atom.Spaces(gap) + st.Muted.Render(ctx) + "\n" + organism.Rule(st, width)
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
