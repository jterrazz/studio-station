package forms

import (
	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/theme"
)

func Styles(isDark bool) *huh.Styles {
	t := theme.New(isDark)
	s := huh.ThemeBase(isDark)
	s.Focused.Title = t.Title
	s.Focused.Description = t.Muted
	s.Focused.ErrorMessage = t.Bad
	s.Focused.ErrorIndicator = t.Bad
	s.Focused.SelectSelector = t.Accent
	s.Focused.Option = t.Plain
	s.Focused.SelectedOption = t.Accent
	s.Focused.FocusedButton = t.Button
	s.Focused.BlurredButton = t.Idle
	s.Blurred.Title = t.Muted
	s.Blurred.Description = t.Subtle
	s.Blurred.FocusedButton = t.Button
	s.Blurred.BlurredButton = t.Idle
	return s
}
