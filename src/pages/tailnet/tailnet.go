package tailnet

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

type PeerJSON struct {
	Name   string   `json:"name"`
	IP     string   `json:"ip"`
	Online bool     `json:"online"`
	Tags   []string `json:"tags"`
}

type ServeJSON struct {
	Port    int    `json:"port"`
	Path    string `json:"path"`
	Target  string `json:"target"`
	Station bool   `json:"station"`
}

type ObjectJSON struct {
	State        string `json:"state"`
	Connected    bool   `json:"connected"`
	BackendState string `json:"backend_state"`
	Name         string `json:"name"`
	DNSName      string `json:"dns_name"`
	IP           string `json:"ip"`
	Variant      string `json:"variant"`
	CLI          string `json:"cli"`
}

type StatusJSON struct {
	Schema string `json:"schema"`
	ObjectJSON
	Peers []PeerJSON  `json:"peers"`
	Serve []ServeJSON `json:"serve"`
}

func ObjectOf(s tailnet.Status) ObjectJSON {
	return ObjectJSON{
		State: string(s.State), Connected: s.Connected, BackendState: s.BackendState,
		Name: s.Name, DNSName: s.DNSName, IP: s.IP, Variant: s.Variant, CLI: s.CLI,
	}
}

func StatusContract(s tailnet.Status) StatusJSON {
	body := StatusJSON{Schema: "station.tailnet/v1", ObjectJSON: ObjectOf(s), Peers: make([]PeerJSON, 0, len(s.Peers)), Serve: make([]ServeJSON, 0, len(s.Serve))}
	for _, p := range s.Peers {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		body.Peers = append(body.Peers, PeerJSON{Name: p.Name, IP: p.IP, Online: p.Online, Tags: tags})
	}
	for _, e := range s.Serve {
		body.Serve = append(body.Serve, ServeJSON{Port: e.Port, Path: e.Path, Target: e.Target, Station: e.Station})
	}
	return body
}

func columns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 50}}
}

func Sections(s tailnet.Status) []page.Section {
	rows := Rows(s)
	switch {
	case s.State == catalog.StatePending:
		rows = append(rows, page.Row{ID: "node", Label: "node", Mark: page.Awaiting}, page.Row{ID: "cli", Label: "cli", Mark: page.Awaiting})
	case s.CLI != "":
		rows = append(rows, page.Row{ID: "cli", Label: "cli", Mark: page.OK, Cells: []page.Value{page.Path(s.CLI)}, Detail: page.Text(s.Variant)})
	}
	head := page.Section{ID: "tailnet", Title: "Tailnet", Role: page.Banner, Columns: columns(), Items: itemsOf(rows)}
	if s.State == catalog.StateAbsent {
		return []page.Section{head}
	}
	peers := make([]page.Row, 0, len(s.Peers))
	for _, p := range s.Peers {
		state := page.Text("online")
		if !p.Online {
			state = page.Text("offline")
		}
		peers = append(peers, page.Row{ID: p.Name, Label: p.Name, Cells: []page.Value{page.Identity(p.IP), state, page.Texts(p.Tags...)}})
	}
	serve := make([]page.Row, 0, len(s.Serve))
	for _, e := range s.Serve {
		owner := "not station's"
		if e.Station {
			owner = "station"
		}
		label := "https " + strconv.Itoa(e.Port) + e.Path
		serve = append(serve, page.Row{ID: label, Label: label, Mark: page.OK, Cells: []page.Value{page.URL(e.Target)}, Detail: page.Text(owner)})
	}
	return []page.Section{head,
		{ID: "peers", Title: "Peers", Columns: []page.Column{
			{Key: "name", Kind: page.KindListing, Keep: 100}, {Key: "ip", Kind: page.KindIdentity, Keep: 60},
			{Key: "state", Kind: page.KindText, Keep: 70, Show: page.EyeOnly}, {Key: "tags", Kind: page.KindList, Keep: 20, Whole: true, Show: page.EyeOnly},
		}, Items: listOf(s, "peers", peers)},
		{ID: "serve", Title: "Serve", Columns: columns(), Items: listOf(s, "serves", serve)},
	}
}

func itemsOf(rows []page.Row) []page.Item {
	items := make([]page.Item, 0, len(rows))
	for i := range rows {
		items = append(items, page.Item{Row: &rows[i]})
	}
	return items
}

func listOf(s tailnet.Status, noun string, rows []page.Row) []page.Item {
	switch {
	case s.State == catalog.StatePending:
		return []page.Item{{Row: &page.Row{ID: noun, Label: "reading", Mark: page.Awaiting}}}
	case len(rows) == 0:
		return []page.Item{{Empty: &page.Empty{Noun: noun}}}
	}
	return itemsOf(rows)
}

func Status(s tailnet.Status, here page.Context) page.Page {
	var hints []page.Hint
	if h := repair(s); h != nil {
		hints = append(hints, *h)
	}
	return page.Seal(page.Page{ID: "tailnet", Command: "tailnet", Context: here, Sections: Sections(s), Hints: hints})
}

func Rows(s tailnet.Status) []page.Row {
	if s.State == catalog.StatePending {
		return []page.Row{{ID: "tailnet", Label: "tailnet", Mark: page.Awaiting}}
	}
	head := page.Row{ID: "tailnet", Label: "tailnet", Mark: pages.MarkOf(s.State), Cells: []page.Value{stateValue(s)}, Hint: repair(s)}
	switch head.Mark {
	case page.Attention:
		head.Reason = page.Degraded
	case page.Broken:
		head.Reason = page.Down
	case page.Absent:
		head.Reason = page.Missing
	case page.OK, page.Awaiting, page.Unjudged:
	}
	rows := []page.Row{head}
	if s.Name != "" {
		rows = append(rows, page.Row{ID: "node", Label: "node", Mark: page.OK, Cells: []page.Value{page.Text(s.Name)}, Detail: page.Text(s.IP)})
	}
	return rows
}

func stateValue(s tailnet.Status) page.Value {
	switch {
	case s.State == catalog.StateAbsent:
		return page.Text("not installed")
	case s.BackendState == "":
		return page.Text("the Tailscale app does not answer")
	}
	return page.Texts(s.BackendState, s.DNSName)
}

func Summary(s tailnet.Status) page.Row {
	row := Rows(s)[0]
	if s.State != catalog.StateOK {
		return row
	}
	row.Cells = []page.Value{page.Texts(s.BackendState, s.DNSName, s.IP, s.Variant, s.CLI)}
	return row
}

func repair(s tailnet.Status) *page.Hint {
	switch s.State {
	case catalog.StateOK, catalog.StatePending:
		return nil
	case catalog.StateAbsent:
		return page.Cmd("station tools install tailscale")
	case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown:
	}
	return &page.Hint{Command: "open the Tailscale app and log in", External: true}
}
