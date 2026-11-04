package ports

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	"github.com/jterrazz/studio-station/src/ports"
)

const KeyStopSharing = "x"

func Columns() []page.Column {
	return []page.Column{
		{Key: "vm", Kind: page.KindText, Keep: 100},
		{Key: "port", Label: "port", Kind: page.KindPortMap, Keep: 80},
		{Key: "url", Label: "url", Kind: page.KindURL, Keep: 40},
		{Key: "state", Label: "state", Kind: page.KindText, Keep: 70},
		{Key: "process", Label: "process", Kind: page.KindText, Keep: 20, Whole: true},
	}
}

func List(l ports.Listing, here page.Context) page.Page {
	sections := Sections(l.Fleet, here)
	if line := TailnetLine(l.Tailnet); line != nil {
		sections = append([]page.Section{*line}, sections...)
	}
	return page.Seal(page.Page{ID: "ports", Command: "ports", Context: here, Sections: sections, Actions: Actions(l.Fleet)})
}

func TailnetLine(s ports.Status) *page.Section {
	// value: the tailnet's line a list of ports begins with: whether the
	// tailnet carries this Mac's shares; nil inside a VM, whose Mac says it.
	if s.State == "" {
		return nil
	}
	row := page.Row{ID: "tailnet", Label: catalog.TailnetProduct}
	switch {
	case s.State == catalog.StatePending:
		row.Mark = page.Awaiting
	case s.State == catalog.StateAbsent:
		row.Mark, row.Reason, row.Cells = page.Absent, page.Missing, []page.Value{page.Text("not installed")}
		row.Hint = repair(s)
	case !s.Connected:
		row.Mark, row.Reason = page.Broken, page.Down
		row.Cells = []page.Value{page.Text("signed out on This Mac: ports are local only")}
		row.Hint = repair(s)
	case !s.ServeOn:
		row.Mark, row.Reason = page.Attention, page.Degraded
		row.Cells = []page.Value{page.Texts("signed in", s.DNSName, "ports are local only")}
		row.Detail = page.Text(catalog.ServeOffDetail)
	default:
		row.Mark, row.Cells = page.OK, []page.Value{page.Texts("signed in", s.DNSName, "ports are shared")}
	}
	return &page.Section{ID: "tailnet", Title: catalog.TailnetProduct, Role: page.Banner,
		Columns: []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "state", Kind: page.KindText, Keep: 90}},
		Items:   []page.Item{{Row: &row}}}
}

func Sections(f ports.Fleet, here page.Context) []page.Section {
	out := make([]page.Section, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		var items []page.Item
		switch {
		case !h.Reachable:
			row := page.Row{ID: h.Name, Label: h.Name, Mark: page.Attention, Reason: page.Unreachable,
				Cells: []page.Value{page.None(), page.None(), page.Text("does not answer")},
				Hint:  page.Cmd("station machines")}
			items = append(items, page.Item{Row: &row})
		case h.Here && !h.Daemon.Running:
			reason, word, fix := page.Stopped, "not shared while the engine is stopped", "station engine start"
			if !h.Daemon.Installed {
				reason, word, fix = page.Missing, "the engine is not installed", "station setup"
			}
			row := page.Row{ID: "engine", Label: "engine", Mark: page.Attention, Reason: reason,
				Cells: []page.Value{page.None(), page.None(), page.Text(word)},
				Hint:  page.Cmd(fix)}
			items = append(items, page.Item{Row: &row})
		}
		if h.Reachable {
			for _, s := range h.Services {
				row := Row(s)
				items = append(items, page.Item{Row: &row})
			}
		}
		if len(items) == 0 {
			hint := page.Cmd("station machines create")
			if !h.Here {
				hint = page.Cmd("station machines create --on " + h.Name)
			}
			items = append(items, page.Item{Empty: &page.Empty{Noun: "shared ports", Hint: hint}})
		}
		title := h.Name
		if title == "" && h.Here {
			title = here.Machine
		}
		out = append(out, page.Section{ID: "host-" + title, Title: title, Role: page.Banner, Caption: Caption(h), Columns: Columns(), Items: items})
	}
	if len(out) == 0 {
		out = append(out, page.Section{ID: "ports", Title: "Ports", Role: page.Banner, Columns: Columns(),
			Items: []page.Item{{Empty: &page.Empty{Noun: "shared ports", Hint: page.Cmd("station machines create")}}}})
	}
	return out
}

func Caption(h ports.HostServices) []page.Value {
	if h.Range == nil {
		return nil
	}
	r := h.Range
	return []page.Value{
		page.Range(int64(r.Low), int64(r.High)), page.Count(int64(r.Leased), "reserved"),
		page.Count(int64(r.Free), "free"), page.Count(int64(r.MaxPerVM), "per VM"),
	}
}

func Actions(f ports.Fleet) []page.Action {
	// invariant: stop sharing binds only a port's row, never a Mac's or the engine's.
	var shared []string
	for _, h := range f.Hosts {
		if !h.Reachable {
			continue
		}
		for _, s := range h.Services {
			shared = append(shared, Row(s).ID)
		}
	}
	if len(shared) == 0 {
		return nil
	}
	return []page.Action{{Key: KeyStopSharing, Label: "stop sharing", On: page.OnRow, Rows: shared, Priority: 1, Mutates: true, Confirm: &page.Confirm{Yes: "stop sharing", Danger: true}}}
}

func StopConfirm(vm string, port int) page.Confirm {
	return page.Confirm{Question: "Stop sharing " + vm + ":" + strconv.Itoa(port) + "?", Lines: []page.Value{page.Text("it is not shared again when it listens, until you share it")}, Danger: true, Yes: "stop sharing"}
}

// value: a port's state in the owner's words (SPEC 5.6), the mark it reads
// with, and the fix it waits for.
func stateOf(s ports.ServiceView) (page.Mark, page.Reason, page.Value, *page.Hint) {
	reserved := "reserved " + strconv.Itoa(s.HostPort)
	switch {
	case s.State == ports.PhaseRefused || s.HostPort == 0:
		return page.Attention, page.Waiting, page.Texts("waiting", s.Detail), page.Cmd("station ports stop-sharing " + s.VM + " <port>")
	case s.State == ports.PhaseUnknown:
		return page.Attention, page.Late, page.Texts("not shared while the engine is stopped", reserved), nil
	case s.State == ports.PhaseWaiting:
		return page.Absent, page.Waiting, page.Texts("not listening", reserved), nil
	case s.Detail != "":
		return page.Attention, page.Degraded, page.Texts("local only", s.Detail), nil
	}
	return page.OK, "", page.Text("shared"), nil
}

func rowID(vm string, port int) string { return vm + ":" + strconv.Itoa(port) }

func Row(s ports.ServiceView) page.Row {
	url := page.URL(s.URLs.Public)
	if s.HostPort == 0 || s.URLs.Public == "" {
		url = page.None()
	}
	port := page.Port(int64(s.GuestPort))
	if s.HostPort != 0 {
		port = page.PortMap(int64(s.GuestPort), int64(s.HostPort))
	}
	mark, reason, state, hint := stateOf(s)
	return page.Row{ID: rowID(s.VM, s.GuestPort), Label: s.VM, Mark: mark, Reason: reason,
		Cells:  []page.Value{port, url, state, page.Text(s.Process)},
		Detail: page.Text(s.Profile), Hint: hint}
}

func Link(s ports.ServiceView) page.Link {
	l := page.Link{Label: strconv.Itoa(s.GuestPort), Lead: page.Port(int64(s.GuestPort)), Value: page.URL(s.URLs.Public), Detail: page.Text(s.Process), Page: "ports", Row: rowID(s.VM, s.GuestPort)}
	if s.HostPort != 0 {
		l.Lead = page.PortMap(int64(s.GuestPort), int64(s.HostPort))
	}
	return l
}

func Shared(s ports.ServiceView, here page.Context) page.Page {
	row := pages.Done(rowID(s.VM, s.GuestPort), catalog.StateOK, "")
	row.Cells = []page.Value{page.URL(s.URLs.Public)}
	return pages.Outcome("ports share "+strconv.Itoa(s.GuestPort), here,
		[]page.Column{{Key: "port", Kind: page.KindText, Keep: 100}, {Key: "url", Kind: page.KindURL, Keep: 90}}, []page.Row{row}, "shared ports")
}

func Stopped(r ports.Reservation, here page.Context) page.Page {
	return pages.Outcome("ports stop-sharing", here, nil, []page.Row{pages.Done(r.Key(), catalog.StateOK, "not shared")}, "shared ports")
}
