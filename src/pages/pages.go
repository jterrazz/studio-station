package pages

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
)

func MarkOf(s catalog.State) page.Mark {
	switch s {
	case catalog.StateOK:
		return page.OK
	case catalog.StateDegraded, catalog.StateUnknown:
		return page.Attention
	case catalog.StateAbsent:
		return page.Absent
	case catalog.StateBroken:
		return page.Broken
	case catalog.StatePending:
		return page.Awaiting
	}
	return page.Unjudged
}

// Failure is the page a read that could not answer at all shows in place of
// its own: the one error block, its message and its hint.
func Failure(id page.ID, command string, here page.Context, message, hint string) page.Page {
	return page.Seal(page.Page{ID: id, Command: command, Context: here, Error: &page.Error{Message: message, Hint: hint}})
}

// Outcome is every mutating leaf's page: one section of what happened, the
// one Empty when nothing was to do, then the leaf's own extra sections.
func Outcome(command string, here page.Context, columns []page.Column, rows []page.Row, empty string, extra ...page.Section) page.Page {
	items := make([]page.Item, 0, len(rows))
	for i := range rows {
		items = append(items, page.Item{Row: &rows[i]})
	}
	if len(items) == 0 {
		items = append(items, page.Item{Empty: &page.Empty{Noun: empty}})
	}
	if len(columns) == 0 {
		columns = OutcomeColumns()
	}
	sections := append([]page.Section{{ID: "changes", Title: "Changes", Role: page.Banner, Columns: columns, Items: items}}, extra...)
	return page.Seal(page.Page{ID: "outcome", Command: command, Context: here, Sections: sections})
}

func OutcomeColumns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}}
}

// Done is an outcome row: the thing acted on, its mark, what happened.
func Done(id string, state catalog.State, detail string) page.Row {
	r := page.Row{ID: id, Label: id, Mark: MarkOf(state), Detail: page.Text(detail)}
	switch r.Mark {
	case page.Attention:
		r.Reason = page.Partial
	case page.Broken:
		r.Reason = page.Failing
	case page.OK, page.Absent, page.Awaiting, page.Unjudged:
	}
	return r
}
