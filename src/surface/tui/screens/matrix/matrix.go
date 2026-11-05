package matrixscreen

import (
	"slices"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	pagesettings "github.com/jterrazz/studio-station/src/pages/settings"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

const (
	// value: the view of the matrix that keeps only the rows needing a look.
	Look = "needs-a-look"
)

func Gesture(a page.Action, rows []page.Row, machine string) (gesture.Gesture, bool) {
	// invariant: a cell is a setting on a machine: enter opens that setting on
	// that machine's own Settings, a applies it on every machine it lives on
	// (the command skips a server and a stopped VM, and says so), F keeps what
	// needs a look, f opens a checked setting's System Settings pane.
	if len(rows) != 1 && a.On == page.OnRow {
		return gesture.Gesture{}, false
	}
	switch a.Key {
	case pagesettings.KeyApply:
		name := rows[0].ID
		return gesture.Gesture{Steps: []gesture.Step{{
			Words: []string{"settings", "apply", name, "--all-machines"},
			Ask:   "Apply " + name + " on every machine?",
			Doing: "Applying " + name + " on every machine",
			Done:  "Applied " + name + " on every machine",
		}}}, true
	case pagesettings.KeyOnMachine:
		return gesture.Gesture{Visit: &gesture.Visit{Machine: machine, Section: "settings", Row: rows[0].ID}}, machine != ""
	case pagesettings.KeyLook:
		return gesture.Gesture{View: Look}, true
	case pagesettings.KeyFix:
		for _, c := range catalog.CheckedSettings {
			if c.Name == rows[0].ID && c.Pane != "" {
				return gesture.Gesture{Open: c.Pane}, true
			}
		}
	}
	return gesture.Gesture{}, false
}

func Shown(p page.Page, look bool) page.Page {
	// value: the page as the view shows it: with Look, only the rows of the
	// setting groups that need a look, a group left with none dropped.
	if !look {
		return p
	}
	out := p
	out.Sections = nil
	for _, sec := range p.Sections {
		if !grid(sec) {
			out.Sections = append(out.Sections, sec)
			continue
		}
		sec.Items = slices.DeleteFunc(slices.Clone(sec.Items), func(it page.Item) bool {
			return it.Row == nil || it.Row.Mark != page.Attention
		})
		if len(sec.Items) > 0 {
			out.Sections = append(out.Sections, sec)
		}
	}
	return out
}

func grid(sec page.Section) bool {
	// value: a section of the matrix's grid: one row per setting, one column
	// per machine, then the OK count.
	return len(sec.Columns) > 2 && sec.Columns[len(sec.Columns)-1].Key == "ok"
}
