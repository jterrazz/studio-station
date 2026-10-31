package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Palette struct {
	Text      color.Color
	Muted     color.Color
	Subtle    color.Color
	Surface   color.Color
	Selection color.Color
	Accent    color.Color
	Good      color.Color
	Warn      color.Color
	Bad       color.Color
}

func Tokens(isDark bool) Palette {
	// value: Catppuccin Latte (light) and Macchiato (dark), the flavor the
	// embedded dotfiles already paint the terminal with.
	pick := lipgloss.LightDark(isDark)
	pair := func(light, dark string) color.Color {
		return pick(lipgloss.Color(light), lipgloss.Color(dark))
	}
	return Palette{
		Text:      pair("#4c4f69", "#cad3f5"),
		Muted:     pair("#6c6f85", "#a5adcb"),
		Subtle:    pair("#9ca0b0", "#6e738d"),
		Surface:   pair("#ccd0da", "#363a4f"),
		Selection: pair("#bcc0cc", "#494d64"),
		Accent:    pair("#1e66f5", "#8aadf4"),
		Good:      pair("#40a02b", "#a6da95"),
		Warn:      pair("#df8e1d", "#eed49f"),
		Bad:       pair("#d20f39", "#ed8796"),
	}
}
