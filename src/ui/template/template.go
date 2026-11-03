package template

import (
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/molecule"
	"github.com/jterrazz/studio-station/src/ui/organism"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func Document(st token.Styles, p page.Page, width int) []string {
	lines := organism.Header(st, p.Command, organism.Context(p.Context), width)
	if p.Error != nil {
		lines = append(lines, "")
		for _, l := range molecule.ErrorLines(st, *p.Error) {
			lines = append(lines, atom.Spaces(token.Margin)+l)
		}
		return lines
	}
	for _, s := range p.Sections {
		lines = append(lines, "")
		lines = append(lines, organism.Section(st, s, width)...)
	}
	if len(p.Hints) > 0 {
		lines = append(lines, "")
		for _, h := range p.Hints {
			lines = append(lines, atom.Spaces(token.Margin)+molecule.PageHint(st, h))
		}
	}
	return lines
}
