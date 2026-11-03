package home

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func backgroundColumns() []page.Column {
	return []page.Column{
		{Key: "label", Kind: page.KindText, Keep: 100},
		{Key: "group", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "state", Kind: page.KindList, Keep: 90},
		{Key: "scope", Kind: page.KindText, Keep: 40},
		{Key: "owner", Kind: page.KindText, Keep: 30},
	}
}

func groups() []status.JobGroup {
	return []status.JobGroup{status.GroupStation, status.GroupKnown, status.GroupUnknown, status.GroupOrphan}
}

// invariant: the station group is always drawn, holding the Empty when this
// Mac has no station agent, so its absence is seen rather than guessed.
func backgroundSection(jobs []status.Job) page.Section {
	var items []page.Item
	for _, g := range groups() {
		var children []page.Item
		for _, j := range jobs {
			if j.Group == g {
				r := jobRow(j)
				children = append(children, page.Item{Row: &r})
			}
		}
		if len(children) == 0 {
			if g != status.GroupStation {
				continue
			}
			children = append(children, page.Item{Empty: &page.Empty{Noun: "background jobs"}})
		}
		items = append(items, page.Item{Group: &page.Group{Head: page.Row{ID: "group/" + string(g), Label: string(g)}, Items: children, Fold: page.Open}})
	}
	return page.Section{ID: "background", Title: "Background", Columns: backgroundColumns(), Items: items, PipeCounts: true}
}

func runOf(j status.Job) page.Value {
	switch j.Run {
	case status.JobRunning:
		return page.Texts("running", "pid "+strconv.Itoa(j.PID))
	case status.JobFailing:
		return page.Texts("failing", "last exit "+strconv.Itoa(j.Exit))
	case status.JobUnread:
		return page.Text("?")
	case status.JobIdle, status.JobNotLoaded:
	}
	if j.Group == status.GroupStation && j.Run == status.JobNotLoaded {
		return page.Text("stopped")
	}
	return page.Text(string(j.Run))
}

func jobRow(j status.Job) page.Row {
	r := page.Row{ID: j.Label, Label: j.Label, Mark: pages.MarkOf(j.State),
		Cells: []page.Value{page.Text(string(j.Group)), runOf(j), page.Text(j.Scope), page.Text(j.Owner)}}
	if j.Plist != "" {
		r.Detail = page.Path(j.Plist)
	}
	if owner, ok := catalog.LaunchOwnerOf(j.Label); ok {
		if cut := strings.LastIndexByte(owner.Prefix, '.'); cut > 0 && cut+1 < len(j.Label) {
			r.Short = j.Label[cut+1:]
		}
	}
	switch {
	case j.Run == status.JobFailing:
		r.Reason = page.Failing
	case j.Group == status.GroupOrphan:
		r.Reason = page.Orphan
	case j.Run == status.JobUnread:
		r.Reason = page.Late
	case j.Group == status.GroupStation && j.Run == status.JobNotLoaded:
		r.Reason = page.Stopped
	}
	switch {
	case j.Removal != "":
		r.Hint = &page.Hint{Command: j.Removal, External: true}
	case j.Plist == "" && r.Reason != "":
		r.Detail = page.Text("managed by " + j.Owner + ", remove it from the app")
	case j.Group == status.GroupStation && j.Run == status.JobNotLoaded:
		r.Hint = page.Cmd("station setup")
	}
	return r
}

// value: the To do label drops a reverse-DNS vendor (`com.jterrazz.`) of a
// long label; the full one stays in the Background section, the pipe and json.
func shortLabel(label string) string {
	parts := strings.Split(label, ".")
	if len(parts) >= 4 {
		return strings.Join(parts[2:], ".")
	}
	return label
}

func jobAction(j status.Job) page.Row {
	r := jobRow(j)
	r.ID, r.Label, r.Short, r.Cells = "background/"+j.Label, shortLabel(j.Label), "", nil
	switch {
	case j.Run == status.JobFailing:
		r.Detail = runOf(j)
	case j.Group == status.GroupOrphan && j.Owner != "":
		r.Detail = page.Text("orphan: " + j.Owner + " is gone")
	case j.Group == status.GroupOrphan:
		r.Detail = page.Text("orphan: its program is missing")
	case j.Plist == "":
	default:
		r.Detail = runOf(j)
	}
	return r
}

func backgroundValue(jobs []status.Job) page.Value {
	failing, orphans := 0, 0
	for _, j := range jobs {
		if j.Run == status.JobFailing {
			failing++
		}
		if j.Group == status.GroupOrphan {
			orphans++
		}
	}
	unit := "jobs"
	if len(jobs) == 1 {
		unit = "job"
	}
	parts := []page.Value{page.Count(int64(len(jobs)), unit)}
	if failing > 0 {
		parts = append(parts, page.Count(int64(failing), "failing"))
	}
	switch {
	case orphans == 1:
		parts = append(parts, page.Count(1, "orphan"))
	case orphans > 1:
		parts = append(parts, page.Count(int64(orphans), "orphans"))
	}
	return page.List(parts...)
}

func backgroundMark(jobs []status.Job) page.Mark {
	m := page.OK
	for _, j := range jobs {
		switch j.State {
		case catalog.StateBroken:
			return page.Broken
		case catalog.StateDegraded, catalog.StateUnknown:
			m = page.Attention
		case catalog.StateOK, catalog.StateAbsent, catalog.StatePending:
		}
	}
	return m
}
