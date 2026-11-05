package gesture

import "github.com/jterrazz/studio-station/src/page"

type Step struct {
	// value: one command an action runs, and how the Progress strip says it.
	Words []string
	// value: the question put when the command asks first (Remove lazygit?).
	Ask         string
	Doing, Done string
}

type Verb struct {
	// value: a gesture's verb, as the question, the running line and the toast
	// say it.
	Do, Doing, Done string
}

func V(do, doing, done string) Verb { return Verb{Do: do, Doing: doing, Done: done} }

func (v Verb) Step(what string, words []string) Step {
	return Step{Words: words, Ask: v.Do + " " + what + "?", Doing: v.Doing + " " + what, Done: v.Done + " " + what}
}

type Gesture struct {
	// value: what an action does once its key is pressed on its rows: the
	// commands it runs, one after the other, or what it shows, copies or opens.
	Steps []Step
	// value: the page the detail pane shows (a row's full page, a diff, a
	// folder's own rows).
	Read []string
	// value: lines the detail pane shows under Title (Fix when none); nothing
	// runs (a fix that is yours, a folder's own rows).
	Note  []string
	Title string
	Copy  string
	// value: a URL or a file opened with its app (a System Settings pane).
	Open string
	// value: a line a new terminal runs.
	Terminal string
	// value: a choice put first; the gesture of the option picked follows.
	Pick *Pick
	// value: a way of showing the section, turned on or off; nothing runs
	// (Disk sorted by name, the matrix's only what needs a look).
	View string
	// value: another machine's section shown, its cursor on a row.
	Visit *Visit
}

type Visit struct {
	Machine, Section, Row string
}

type Pick struct {
	Title   string
	Options []string
	Then    func(option string) Gesture
}

func Each(rows []page.Row, v Verb, words func(r page.Row) []string) Gesture {
	// value: one step per row, each its own command.
	var g Gesture
	for _, r := range rows {
		g.Steps = append(g.Steps, v.Step(r.Label, words(r)))
	}
	return g
}

func Cell(sec page.Section, r page.Row, key string) page.Value {
	// value: the text of a row's cell by its column's key.
	for i, c := range sec.Columns {
		if c.Key == key && i > 0 && i-1 < len(r.Cells) {
			return r.Cells[i-1]
		}
	}
	return page.None()
}
