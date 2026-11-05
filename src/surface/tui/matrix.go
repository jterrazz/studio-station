package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	diskscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/disk"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
	matrixscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/matrix"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: All machines › Settings is the matrix: its cursor is on a cell,
// a setting on a machine (SPEC §9.4).
func (s shell) matrix() bool { return s.current().all() && s.section() == section.Settings }

// value: the matrix as the view shows it, its cursor kept on a shown row.
func (s shell) cell(key string) ui.Matrix {
	m := s.cells[key]
	p, _ := s.pageOf(key)
	m.Page = matrixscreen.Shown(p, s.views[key][matrixscreen.Look])
	m.Filter = s.lists[key].Filter
	if s.views[key][matrixscreen.Look] {
		m.View = "only what needs a look"
	}
	return m.Fix()
}

// value: the row the focused pane's cursor is on: the matrix's cell row, or
// the list's.
func (s shell) focused(key string) (page.Row, bool) {
	if s.matrix() && key == s.key(s.current(), s.section()) {
		r, _, ok := s.cell(key).Current()
		return r, ok
	}
	_, r, ok := s.list(key).Current()
	return r, ok
}

func (s shell) moveCell(rows, cols int) (shell, bool) {
	k := s.key(s.current(), s.section())
	m := s.cell(k)
	if cols < 0 && m.Col == 0 {
		return s, false
	}
	m = m.Move(rows, cols)
	s.cells = with(s.cells, k, ui.Matrix{Row: m.Row, Col: m.Col})
	return s, true
}

// value: the page a section's list shows: the read itself, or the view
// of it a key turned on (Disk sorted by name).
func (s shell) viewed(key string, l ui.List) ui.List {
	n, id, ok := s.parse(key)
	if !ok || id != section.Disk || n.all() {
		return l
	}
	p, read := s.pageOf(key)
	if !read {
		return l
	}
	shown, _ := diskscreen.Shown(p, s.views[key][diskscreen.ByName])
	return l.Show(shown)
}

func (s shell) viewTitle(key string) string {
	n, id, ok := s.parse(key)
	if !ok || id != section.Disk || n.all() {
		return ""
	}
	_, words := diskscreen.Shown(page.Page{}, s.views[key][diskscreen.ByName])
	return words
}

// invariant: a visit shows the machine's own section with the cursor on
// the row named, once its page has answered.
func (s shell) visit(v gesture.Visit) (shell, tea.Cmd) {
	n, ok := find(s.nodes, v.Machine)
	if !ok {
		return s, nil
	}
	s.at = n.name
	s.sec = max(slices.Index(s.sections(n), section.ID(v.Section)), 0)
	s.side = s.sectionEntry()
	s.focus = ui.PaneMaster
	k := s.key(n, s.section())
	s.want = with(s.want, k, v.Row)
	s = s.reveal(k)
	m, cmd := s.load()
	return m, cmd
}

func (s shell) reveal(key string) shell {
	row, ok := s.want[key]
	if !ok {
		return s
	}
	if _, read := s.pageOf(key); !read {
		return s
	}
	if l, found := s.list(key).Reveal(row); found {
		s.lists = with(s.lists, key, l)
	}
	want := map[string]string{}
	for k, v := range s.want {
		if k != key {
			want[k] = v
		}
	}
	s.want = want
	return s
}

// value: n makes a workspace on the Mac shown, or the Mac of the VM shown,
// in the Progress strip (SPEC §9.3).
func (s shell) create() (tea.Model, tea.Cmd) {
	n := s.current()
	mac := n.mac
	switch {
	case n.all():
		mac = s.deps.Here.Machine
	case n.kind == machine.Mac:
		mac = n.name
	}
	words := []string{"machines", "create"}
	what := "a workspace"
	if m, ok := find(s.nodes, mac); ok && !m.here {
		words, what = append(words, "--on", mac), what+" on "+m.label
	}
	st := gesture.Step{Words: words, Doing: "Creating " + what, Done: "Created " + what}
	return s.start(task{step: st, n: 1, total: 1, stale: s.stale()}, Answer{})
}

// value: the keys the shown section answers, with their labels, so an
// empty line names the one that fills it.
func (s shell) bound() func(key string) (string, bool) {
	var labels map[string]string
	return func(key string) (string, bool) {
		if labels == nil {
			labels = map[string]string{"n": "new VM"}
			for _, b := range s.actionKeys() {
				labels[b.Key] = b.Label
			}
		}
		label, ok := labels[key]
		return label, ok
	}
}
