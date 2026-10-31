package theme

import "charm.land/lipgloss/v2"

type Theme struct {
	Tokens Palette
	Plain  lipgloss.Style
	Strong lipgloss.Style
	Muted  lipgloss.Style
	Subtle lipgloss.Style
	Accent lipgloss.Style
	Title  lipgloss.Style
	Good   lipgloss.Style
	Warn   lipgloss.Style
	Bad    lipgloss.Style
	Error  lipgloss.Style
	Button lipgloss.Style
	Idle   lipgloss.Style
}

func New(isDark bool) Theme {
	t := Tokens(isDark)
	plain := lipgloss.NewStyle().Foreground(t.Text)
	accent := lipgloss.NewStyle().Foreground(t.Accent)
	bad := lipgloss.NewStyle().Foreground(t.Bad)
	return Theme{
		Tokens: t,
		Plain:  plain,
		Strong: plain.Bold(true),
		Muted:  lipgloss.NewStyle().Foreground(t.Muted),
		Subtle: lipgloss.NewStyle().Foreground(t.Subtle),
		Accent: accent,
		Title:  accent.Bold(true),
		Good:   lipgloss.NewStyle().Foreground(t.Good),
		Warn:   lipgloss.NewStyle().Foreground(t.Warn),
		Bad:    bad,
		Error:  bad.Bold(true),
		Button: lipgloss.NewStyle().Foreground(t.Surface).Background(t.Accent).Bold(true).Padding(0, 1),
		Idle:   lipgloss.NewStyle().Foreground(t.Muted).Background(t.Surface).Padding(0, 1),
	}
}
