package diskscreen

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	pagedisk "github.com/jterrazz/studio-station/src/pages/disk"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

const (
	// value: the view of Disk that lists each group's rows by name; off,
	// they stay by size, the largest first, as the page lists them.
	ByName = "by-name"
)

// value: a disk row is `<group>/<what>`: a folder for Misplaced, an app
// for every other group.
func what(r page.Row) string {
	_, w, _ := strings.Cut(r.ID, "/")
	return w
}

func Gesture(p page.Page, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	var g gesture.Gesture
	switch a.Key {
	case pagedisk.KeyMove:
		g = gesture.Each(rows, gesture.V("Move", "Moving", "Moved"), func(r page.Row) []string { return []string{"disk", "move", what(r)} })
	case pagedisk.KeyFree:
		g = gesture.Each(rows, gesture.V("Free", "Freeing", "Freed"), func(r page.Row) []string { return []string{"disk", "free", what(r)} })
	case pagedisk.KeyErase:
		g = gesture.Each(rows, gesture.V("Erase", "Erasing", "Erased"), func(r page.Row) []string { return []string{"disk", "erase", what(r)} })
	case pagedisk.KeySort:
		return gesture.Gesture{View: ByName}, true
	case pagedisk.KeyOpen:
		return open(p, rows)
	default:
		return g, false
	}
	return g, len(g.Steps) > 0
}

// invariant: enter opens the row's folders, each its own line: what the
// row's size is made of, where it lives on disk.
func open(p page.Page, rows []page.Row) (gesture.Gesture, bool) {
	if len(rows) != 1 {
		return gesture.Gesture{}, false
	}
	r := rows[0]
	group, _, _ := strings.Cut(r.ID, "/")
	for _, sec := range p.Sections {
		if sec.ID != group {
			continue
		}
		var lines []string
		for _, v := range gesture.Cell(sec, r, "folders").Items {
			lines = append(lines, v.Text)
		}
		return gesture.Gesture{Title: r.Label, Note: lines}, len(lines) > 0
	}
	return gesture.Gesture{}, false
}

func Shown(p page.Page, byName bool) (shown page.Page, words string) {
	// value: the page as the view shows it, and the words its title adds.
	if !byName {
		return p, "sorted by size"
	}
	out := p
	out.Sections = slices.Clone(p.Sections)
	for i, sec := range out.Sections {
		if sec.Role == page.Banner {
			continue
		}
		items := slices.Clone(sec.Items)
		slices.SortStableFunc(items, func(a, b page.Item) int {
			if a.Row == nil || b.Row == nil {
				return 0
			}
			return strings.Compare(strings.ToLower(a.Row.Label), strings.ToLower(b.Row.Label))
		})
		out.Sections[i].Items = items
	}
	return out, "sorted by name"
}
