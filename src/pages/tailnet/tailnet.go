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

// Sections: the node, then its peers (an unjudged listing: online is a value)
// and what it serves, each titled.
func Sections(s tailnet.Status) []page.Section {
	rows := Rows(s)
	if s.CLI != "" {
		rows = append(rows, page.Row{ID: "cli", Label: "cli", Mark: page.OK, Cells: []page.Value{page.Path(s.CLI)}, Detail: page.Text(s.Variant)})
	}
	items := make([]page.Item, 0, len(rows))
	for i := range rows {
		items = append(items, page.Item{Row: &rows[i]})
	}
	out := []page.Section{{ID: "tailnet", Title: "Tailnet", Role: page.Banner, Columns: columns(), Items: items}}
	if len(s.Peers) > 0 {
		peers := make([]page.Item, 0, len(s.Peers))
		for _, p := range s.Peers {
			state := page.Text("online")
			if !p.Online {
				state = page.Text("offline")
			}
			row := page.Row{ID: p.Name, Label: p.Name, Cells: []page.Value{page.Identity(p.IP), state}, Detail: page.Texts(p.Tags...)}
			peers = append(peers, page.Item{Row: &row})
		}
		out = append(out, page.Section{ID: "peers", Title: "Peers", Columns: []page.Column{
			{Key: "name", Kind: page.KindListing, Keep: 100}, {Key: "ip", Kind: page.KindIdentity, Keep: 60}, {Key: "state", Kind: page.KindText, Keep: 70},
		}, Items: peers})
	}
	if len(s.Serve) > 0 {
		serve := make([]page.Item, 0, len(s.Serve))
		for _, e := range s.Serve {
			owner := "not station's"
			if e.Station {
				owner = "station"
			}
			label := "https " + strconv.Itoa(e.Port) + e.Path
			row := page.Row{ID: label, Label: label, Mark: page.OK, Cells: []page.Value{page.URL(e.Target)}, Detail: page.Text(owner)}
			serve = append(serve, page.Item{Row: &row})
		}
		out = append(out, page.Section{ID: "serve", Title: "Serve", Columns: columns(), Items: serve})
	}
	return out
}

func Status(s tailnet.Status, here page.Context) page.Page {
	var hints []page.Hint
	if h := repair(s); h != nil {
		hints = append(hints, *h)
	}
	return page.Seal(page.Page{ID: "tailnet", Command: "tailnet", Context: here, Sections: Sections(s), Hints: hints})
}

func Rows(s tailnet.Status) []page.Row {
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

// Summary is the one tailnet row the status page carries: the state, then the
// DNS name, IP, variant and CLI path as one list, which truncates whole items.
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
	case catalog.StateOK:
		return nil
	case catalog.StateAbsent:
		return page.Cmd("station tools install tailscale")
	case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown, catalog.StatePending:
	}
	return &page.Hint{Command: "open the Tailscale app and log in", External: true}
}
