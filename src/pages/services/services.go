package services

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const KeyRelease = "x"

func Columns() []page.Column {
	return []page.Column{
		{Key: "vm", Kind: page.KindText, Keep: 100},
		{Key: "port", Label: "port", Kind: page.KindPortMap, Keep: 80},
		{Key: "url", Label: "url", Kind: page.KindURL, Keep: 40},
		{Key: "host", Kind: page.KindText, Show: page.PipeOnly},
		{Key: "state", Label: "state", Kind: page.KindText, Keep: 70},
		{Key: "process", Label: "process", Kind: page.KindText, Keep: 20},
	}
}

func Sections(f services.Fleet, here page.Context) []page.Section {
	out := make([]page.Section, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		var items []page.Item
		switch {
		case !h.Reachable:
			row := page.Row{ID: h.Name, Label: h.Name, Mark: page.Attention, Reason: page.Unreachable,
				Cells: []page.Value{page.None(), page.None(), page.Text(h.Name), page.Text("unreachable")},
				Hint:  page.Cmd("ssh " + h.Name + " true")}
			items = append(items, page.Item{Row: &row})
		case h.Here && !h.Daemon.Running:
			reason, word, fix := page.Stopped, "stopped", "station start"
			if !h.Daemon.Installed {
				reason, word, fix = page.Missing, "not installed", "station setup"
			}
			row := page.Row{ID: "daemon", Label: "daemon", Mark: page.Attention, Reason: reason,
				Cells: []page.Value{page.None(), page.None(), page.Text(h.Name), page.Text(word)},
				Hint:  page.Cmd(fix)}
			items = append(items, page.Item{Row: &row})
		}
		if h.Reachable {
			for _, s := range h.Services {
				row := Row(s, h.Name)
				items = append(items, page.Item{Row: &row})
			}
		}
		if len(items) == 0 {
			hint := page.Cmd("station machines create")
			if !h.Here {
				hint = page.Cmd("station machines create --on " + h.Name)
			}
			items = append(items, page.Item{Empty: &page.Empty{Noun: "services", Hint: hint}})
		}
		title := h.Name
		if title == "" && h.Here {
			title = here.Machine
		}
		out = append(out, page.Section{ID: "host-" + title, Title: title, Role: page.Banner, Caption: Caption(h), Columns: Columns(), Items: items})
	}
	if len(out) == 0 {
		out = append(out, page.Section{ID: "services", Title: "Services", Role: page.Banner, Columns: Columns(),
			Items: []page.Item{{Empty: &page.Empty{Noun: "services", Hint: page.Cmd("station machines create")}}}})
	}
	return out
}

func Caption(h services.HostView) []page.Value {
	if h.Range == nil {
		return nil
	}
	r := h.Range
	return []page.Value{
		page.Range(int64(r.Low), int64(r.High)), page.Count(int64(r.Leased), "leased"),
		page.Count(int64(r.Free), "free"), page.Count(int64(r.MaxPerVM), "per VM"),
	}
}

func Fleet(f services.Fleet, here page.Context) page.Page {
	return page.Seal(page.Page{ID: "services", Command: "services", Context: here, Sections: Sections(f, here), Actions: Actions(f)})
}

func Actions(f services.Fleet) []page.Action {
	// invariant: release binds only a lease's row, never a host's or the daemon's.
	var leases []string
	for _, h := range f.Hosts {
		if !h.Reachable {
			continue
		}
		for _, s := range h.Services {
			leases = append(leases, Row(s, h.Name).ID)
		}
	}
	if len(leases) == 0 {
		return nil
	}
	return []page.Action{{Key: KeyRelease, Label: "release", On: page.OnRow, Rows: leases, Priority: 1, Mutates: true, Confirm: &page.Confirm{Yes: "release", Danger: true}}}
}

func ReleaseConfirm(vm string, port int) page.Confirm {
	return page.Confirm{Question: "Release " + vm + ":" + strconv.Itoa(port) + "?", Lines: []page.Value{page.Text("its URL changes at its next listen")}, Danger: true, Yes: "release"}
}

func markOf(s services.View) (page.Mark, page.Reason) {
	switch {
	case s.State == services.Refused:
		return page.Broken, page.Refused
	case s.State == services.Unknown:
		return page.Attention, page.Late
	case s.State == services.Waiting:
		return page.Absent, page.Waiting
	case s.Detail != "":
		return page.Attention, page.Degraded
	}
	return page.OK, ""
}

func rowID(vm string, port int) string { return vm + ":" + strconv.Itoa(port) }

func Row(s services.View, host string) page.Row {
	public := page.URL(s.URLs.Public)
	if s.HostPort == 0 || s.URLs.Public == "" {
		public = page.None()
	}
	detail := s.Detail
	if detail == "" {
		detail = s.Profile
	}
	port := page.Port(int64(s.GuestPort))
	if s.HostPort != 0 {
		port = page.PortMap(int64(s.GuestPort), int64(s.HostPort))
	}
	mark, reason := markOf(s)
	return page.Row{ID: rowID(s.VM, s.GuestPort), Label: s.VM, Mark: mark, Reason: reason,
		Cells:  []page.Value{port, public, page.Text(host), page.Text(string(s.State)), page.Text(s.Process)},
		Detail: page.Text(detail)}
}

func Link(s services.View) page.Link {
	l := page.Link{Label: strconv.Itoa(s.GuestPort), Value: page.URL(s.URLs.Public), Detail: page.Text(s.Process), Page: "services", Row: rowID(s.VM, s.GuestPort)}
	if s.HostPort != 0 {
		l.Value = page.List(page.PortMap(int64(s.GuestPort), int64(s.HostPort)), page.URL(s.URLs.Public))
	}
	return l
}

func URL(s services.View, here page.Context) page.Page {
	row := pages.Done(rowID("url", s.GuestPort), catalog.StateOK, "")
	row.Label = strconv.Itoa(s.GuestPort)
	row.Cells = []page.Value{page.URL(s.URLs.Public)}
	return pages.Outcome("services url "+strconv.Itoa(s.GuestPort), here,
		[]page.Column{{Key: "port", Kind: page.KindText, Keep: 100}, {Key: "url", Kind: page.KindURL, Keep: 90}}, []page.Row{row}, "URLs")
}

func Released(l services.Lease, here page.Context) page.Page {
	return pages.Outcome("services release", here, nil, []page.Row{pages.Done(l.Key(), catalog.StateOK, "released")}, "services")
}
