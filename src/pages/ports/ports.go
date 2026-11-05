package ports

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	"github.com/jterrazz/studio-station/src/ports"
)

const KeyStopSharing = "x"

const (
	ClassNeedsYou = "needs you"
	ClassPublic   = "public"
	ClassPrivate  = "private"
	ClassLocal    = "this machine only"
)

func Columns() []page.Column {
	return []page.Column{
		{Key: "vm", Kind: page.KindText, Keep: 100},
		{Key: "port", Label: "port", Kind: page.KindPortMap, Keep: 80},
		{Key: "class", Label: "class", Kind: page.KindText, Keep: 85},
		{Key: "url", Label: "url", Kind: page.KindURL, Keep: 40},
		{Key: "state", Label: "state", Kind: page.KindText, Keep: 70},
		{Key: "process", Label: "process", Kind: page.KindText, Keep: 20, Whole: true},
	}
}

func List(l ports.Listing, here page.Context) page.Page { return Listed(l, here, false) }

func Listed(l ports.Listing, here page.Context, all bool) page.Page {
	sections := Sections(l.Fleet, here, all)
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
		row.Cells = []page.Value{page.Text("is signed out on This Mac: ports are local only")}
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

func Sections(f ports.Fleet, here page.Context, all bool) []page.Section {
	// invariant: SPEC 5.6: what needs you first, then Public, then Private;
	// what listens on its machine only is a count, unfolded by all; each Mac's
	// reserved ports one line.
	var red, public, private, local, notes []page.Item
	for _, h := range f.Hosts {
		private = append(private, hostRows(h)...)
		if !h.Reachable {
			continue
		}
		shared := map[string]bool{}
		for _, s := range h.Services {
			row := Row(s)
			shared[row.ID] = true
			private = append(private, page.Item{Row: &row})
		}
		for _, p := range h.Ports {
			switch {
			case p.Red():
				row := RedRow(p)
				red = append(red, page.Item{Row: &row})
			case p.Declared != nil:
				row := PublicRow(p)
				public = append(public, page.Item{Row: &row})
			case shared[p.Key()]:
			case p.Reach == ports.ThisMachine:
				row := LocalRow(p)
				local = append(local, page.Item{Row: &row})
			default:
				row := PrivateRow(p)
				private = append(private, page.Item{Row: &row})
			}
		}
		if h.Range != nil {
			notes = append(notes, page.Item{Note: &page.Note{Text: reserved(h, here)}})
		}
	}
	var out []page.Section
	if len(red) > 0 {
		out = append(out, page.Section{ID: "needs-you", Title: "Needs you: " + catalog.ReachableOutside + ", not declared public", PipeCounts: true, Columns: exposedColumns(), Items: red})
	}
	if len(public) > 0 {
		out = append(out, page.Section{ID: "public", Title: "Public: reachable from anywhere else, internet or LAN, declared", PipeCounts: true, Columns: exposedColumns(), Items: public})
	}
	if len(private) == 0 {
		private = append(private, page.Item{Empty: &page.Empty{Noun: "shared ports", Hint: page.Cmd("station machines create")}})
	}
	out = append(out, page.Section{ID: "private", Title: "Private: shared with you on " + catalog.TailnetProduct, PipeCounts: true, Columns: Columns(), Items: private})
	if all && len(local) > 0 {
		out = append(out, page.Section{ID: "local-only", Title: "Listens on its machine only", PipeCounts: true, Columns: exposedColumns(), Items: local})
	}
	if !all && len(local) > 0 {
		notes = append(notes, page.Item{Note: &page.Note{Text: localOnly(len(local))}})
	}
	if len(notes) > 0 {
		out = append(out, page.Section{ID: "ranges", Title: "Reserved ports", Role: page.Banner, Uncounted: true, Columns: Columns(), Items: notes})
	}
	return out
}

func localOnly(n int) string {
	if n == 1 {
		return "1 more listens on its machine only"
	}
	return strconv.Itoa(n) + " more listen on their machine only"
}

// value: a host's own rows: it does not answer, or its engine does not run.
func hostRows(h ports.HostServices) []page.Item {
	var row page.Row
	switch {
	case !h.Reachable:
		row = page.Row{ID: h.Name, Label: h.Name, Mark: page.Attention, Reason: page.Unreachable,
			Cells: []page.Value{page.None(), page.None(), page.None(), page.Text("does not answer")},
			Hint:  page.Cmd("station machines")}
	case h.Here && !h.Daemon.Running:
		reason, word, fix := page.Stopped, "not shared while the engine is stopped", "station engine start"
		if !h.Daemon.Installed {
			reason, word, fix = page.Missing, "the engine is not installed", "station setup"
		}
		row = page.Row{ID: "engine", Label: "engine", Mark: page.Attention, Reason: reason,
			Cells: []page.Value{page.None(), page.None(), page.None(), page.Text(word)},
			Hint:  page.Cmd(fix)}
	default:
		return nil
	}
	return []page.Item{{Row: &row}}
}

func exposedColumns() []page.Column {
	return []page.Column{
		{Key: "machine", Kind: page.KindText, Keep: 100},
		{Key: "port", Label: "port", Kind: page.KindPort, Keep: 90},
		{Key: "class", Label: "class", Kind: page.KindText, Keep: 85},
		{Key: "what", Label: "what", Kind: page.KindText, Keep: 60},
		{Key: "listens", Label: "listens on", Kind: page.KindText, Keep: 50},
		{Key: "declared", Label: "declared", Kind: page.KindText, Keep: 70},
	}
}

func ListensOn(p ports.Port) page.Value {
	// value: the addresses a port listens on, in the owner's words: every
	// interface, the LAN, the tailnet, this machine.
	words := make([]string, 0, len(p.Binds))
	for _, b := range p.Binds {
		port := strconv.Itoa(p.Port)
		switch addr := strings.Trim(b, "[]"); {
		case addr == "" || addr == "*" || addr == "0.0.0.0" || addr == "::":
			words = append(words, "*:"+port+" all interfaces")
		case ports.ReachOf(b, ports.OnMac) == ports.ThisMachine:
			words = append(words, b+":"+port+" this machine")
		case ports.ReachOf(b, ports.OnMac) == ports.Tailnet:
			words = append(words, b+":"+port+" "+catalog.TailnetProduct)
		default:
			words = append(words, b+":"+port+" LAN")
		}
	}
	return page.Texts(words...)
}

func RedRow(p ports.Port) page.Row {
	row := page.Row{ID: p.Key(), Label: p.Machine.Name, Mark: page.Broken, Reason: page.Exposed,
		Cells: []page.Value{page.Port(int64(p.Port)), page.Text(ClassNeedsYou), page.Text(p.What()), ListensOn(p), page.Text("declared no")}}
	if k, ok := catalog.KnownPortOf(p.Port, p.Process); ok && k.Off != "" {
		row.Hint = page.Cmd(k.Off)
	}
	return row
}

func PublicRow(p ports.Port) page.Row {
	// invariant: SPEC 5.6: a declared port is OK when its route is seen,
	// attention with what is missing when it is not.
	row := page.Row{ID: p.Key(), Label: p.Machine.Name, Mark: page.OK,
		Cells: []page.Value{page.Port(int64(p.Port)), page.Text(ClassPublic), page.Text(p.What()), ListensOn(p), page.Texts("declared", p.Declared.Via)}}
	v := p.Verified
	switch {
	case v == nil:
	case !v.Seen:
		row.Mark, row.Reason = page.Attention, page.Degraded
		row.Cells[4] = page.Text("declared, but " + v.Missing)
	default:
		r, _ := catalog.RouteNamed(p.Declared.Via)
		row.Cells[4] = page.Texts(r.Label, r.Seen)
	}
	return row
}

func LocalRow(p ports.Port) page.Row {
	return page.Row{ID: p.Key(), Label: p.Machine.Name, Mark: page.OK,
		Cells: []page.Value{page.Port(int64(p.Port)), page.Text(ClassLocal), page.Text(p.What()), ListensOn(p), page.None()}}
}

func PrivateRow(p ports.Port) page.Row {
	// value: a port reachable through the tailnet only that station does not
	// share: a VM's own tailnet device, its Mac, or the Mac's tailnet address.
	how := "on " + catalog.TailnetProduct
	switch {
	case p.Device:
		how = "own " + catalog.TailnetProduct + " device: " + p.Machine.Name
	case p.Machine.On != "":
		how = "behind " + p.Machine.On + ", not shared"
	}
	return page.Row{ID: p.Key(), Label: p.Machine.Name, Mark: page.OK,
		Cells: []page.Value{page.Port(int64(p.Port)), page.Text(ClassPrivate), page.None(), page.Text(how), page.Text(p.What())}}
}

func reserved(h ports.HostServices, here page.Context) string {
	r, mac := h.Range, h.Name
	if mac == here.Machine || mac == "" && h.Here {
		mac = "This Mac"
	}
	size := r.High - r.Low + 1
	return "Reserved on " + mac + ": " + strconv.Itoa(r.Leased) + " of " + strconv.Itoa(size) +
		" (" + strconv.Itoa(r.Low) + "-" + strconv.Itoa(r.High) + "), " + strconv.Itoa(r.Free) + " free"
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
	case s.State == ports.PhaseWaiting && strings.HasPrefix(s.Detail, catalog.StoppedDetail(s.VM)):
		return page.Absent, page.Stopped, page.Texts(s.Detail, reserved), nil
	case s.State == ports.PhaseWaiting:
		return page.Absent, page.Waiting, page.Texts("not listening", reserved), nil
	case s.Detail == catalog.IssuingDetail:
		return page.Attention, page.Waiting, page.Text(s.Detail), nil
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
		Cells:  []page.Value{port, page.Text(ClassPrivate), url, state, page.Text(s.Process)},
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

func Declared(d ports.Declaration, here page.Context) page.Page {
	r, _ := catalog.RouteNamed(d.Via)
	row := pages.Done(d.Key(), catalog.StateOK, "declared public ("+r.Label+")")
	row.Label = strconv.Itoa(d.Port) + " on " + d.Machine
	return pages.Outcome("ports declare", here, nil, []page.Row{row}, "declared ports")
}

func Undeclared(d ports.Declaration, here page.Context) page.Page {
	row := pages.Done(d.Key(), catalog.StateOK, "not declared")
	row.Label = strconv.Itoa(d.Port) + " on " + d.Machine
	return pages.Outcome("ports undeclare", here, nil, []page.Row{row}, "declared ports")
}

func Stopped(r ports.Reservation, here page.Context) page.Page {
	return pages.Outcome("ports stop-sharing", here, nil, []page.Row{pages.Done(r.Key(), catalog.StateOK, "not shared")}, "shared ports")
}
