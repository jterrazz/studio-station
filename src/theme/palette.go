package theme

import "github.com/jterrazz/studio-station/src/ui/token"

type Palette = token.Palette

func Tokens(isDark bool) Palette { return token.Colors(isDark) }
