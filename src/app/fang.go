package app

import (
	"image/color"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/theme"
)

func fangScheme(pick lipgloss.LightDarkFunc) fang.ColorScheme {
	light, dark := theme.Tokens(false), theme.Tokens(true)
	token := func(of func(theme.Palette) color.Color) color.Color { return pick(of(light), of(dark)) }
	text := token(func(p theme.Palette) color.Color { return p.Text })
	muted := token(func(p theme.Palette) color.Color { return p.Muted })
	accent := token(func(p theme.Palette) color.Color { return p.Accent })
	good := token(func(p theme.Palette) color.Color { return p.Good })
	warn := token(func(p theme.Palette) color.Color { return p.Warn })
	bad := token(func(p theme.Palette) color.Color { return p.Bad })
	surface := token(func(p theme.Palette) color.Color { return p.Surface })
	return fang.ColorScheme{
		Base:           text,
		Title:          accent,
		Description:    text,
		Codeblock:      surface,
		Program:        accent,
		DimmedArgument: muted,
		Comment:        muted,
		Flag:           good,
		FlagDefault:    muted,
		Command:        accent,
		QuotedString:   warn,
		Argument:       text,
		Help:           muted,
		Dash:           muted,
		ErrorHeader:    [2]color.Color{surface, bad},
		ErrorDetails:   bad,
	}
}
