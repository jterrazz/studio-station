package tailnet

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/present"
)

type peerJSON struct {
	Name   string   `json:"name"`
	IP     string   `json:"ip"`
	Online bool     `json:"online"`
	Tags   []string `json:"tags"`
}

type serveJSON struct {
	Port    int    `json:"port"`
	Path    string `json:"path"`
	Target  string `json:"target"`
	Station bool   `json:"station"`
}

type Object struct {
	State        string `json:"state"`
	Connected    bool   `json:"connected"`
	BackendState string `json:"backend_state"`
	Name         string `json:"name"`
	DNSName      string `json:"dns_name"`
	IP           string `json:"ip"`
	Variant      string `json:"variant"`
	CLI          string `json:"cli"`
}

type statusJSON struct {
	Schema string `json:"schema"`
	Object
	Peers []peerJSON  `json:"peers"`
	Serve []serveJSON `json:"serve"`
}

func ObjectOf(s tailnet.Status) Object {
	return Object{
		State: string(s.State), Connected: s.Connected, BackendState: s.BackendState,
		Name: s.Name, DNSName: s.DNSName, IP: s.IP, Variant: s.Variant, CLI: s.CLI,
	}
}

func Status(s tailnet.Status, context string) present.Answer {
	body := statusJSON{Schema: "station.tailnet/v1", Object: ObjectOf(s), Peers: make([]peerJSON, 0, len(s.Peers)), Serve: make([]serveJSON, 0, len(s.Serve))}
	peers := make([]present.Row, 0, len(s.Peers))
	for _, p := range s.Peers {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		body.Peers = append(body.Peers, peerJSON{Name: p.Name, IP: p.IP, Online: p.Online, Tags: tags})
		mark := present.MarkOK
		if !p.Online {
			mark = present.MarkAbsent
		}
		peers = append(peers, present.Row{Mark: mark, Cells: []present.Cell{
			{Text: p.Name, Tone: present.Strong},
			{Text: p.IP},
		}})
	}
	serve := make([]present.Row, 0, len(s.Serve))
	for _, e := range s.Serve {
		body.Serve = append(body.Serve, serveJSON{Port: e.Port, Path: e.Path, Target: e.Target, Station: e.Station})
		owner := "not station's"
		if e.Station {
			owner = "station"
		}
		serve = append(serve, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: "https " + strconv.Itoa(e.Port) + e.Path, Tone: present.Strong},
			{Text: e.Target},
			{Text: owner, Tone: present.Muted},
		}})
	}
	rows := Rows(s)
	if s.CLI != "" {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: "cli", Tone: present.Strong},
			{Text: s.CLI},
			{Text: s.Variant, Tone: present.Muted},
		}})
	}
	blocks := []present.Block{present.Table{Rows: rows}}
	if len(peers) > 0 {
		blocks = append(blocks, present.Section{Title: "Peers", Blocks: []present.Block{present.Table{Rows: peers}}})
	}
	if len(serve) > 0 {
		blocks = append(blocks, present.Section{Title: "Serve", Blocks: []present.Block{present.Table{Rows: serve}}})
	}
	return present.Answer{
		Doc:  present.Document{Command: "station tailnet", Context: context, Blocks: blocks, Hints: hints(s)},
		JSON: body,
	}
}

func Rows(s tailnet.Status) []present.Row {
	rows := []present.Row{{Mark: present.MarkOf(s.State), Cells: []present.Cell{
		{Text: "tailnet", Tone: present.Strong},
		{Text: stateText(s)},
		{Text: repair(s), Tone: present.Muted},
	}}}
	if s.Name != "" {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: "node", Tone: present.Strong},
			{Text: s.Name},
			{Text: s.IP, Tone: present.Muted},
		}})
	}
	return rows
}

func stateText(s tailnet.Status) string {
	switch {
	case s.State == catalog.StateAbsent:
		return "not installed"
	case s.BackendState == "":
		return "the Tailscale app does not answer"
	case s.DNSName != "":
		return s.BackendState + " · " + s.DNSName
	}
	return s.BackendState
}

func Summary(s tailnet.Status) present.Row {
	row := Rows(s)[0]
	if s.State != catalog.StateOK {
		return row
	}
	parts := []string{s.BackendState}
	for _, part := range []string{s.DNSName, s.IP, s.Variant, s.CLI} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	row.Cells[1].Text = strings.Join(parts, " · ")
	return row
}

func repair(s tailnet.Status) string {
	switch s.State {
	case catalog.StateOK:
		return ""
	case catalog.StateAbsent:
		return "station tools install tailscale"
	case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown, catalog.StatePending:
	}
	return "open the Tailscale app and log in"
}

func hints(s tailnet.Status) []string {
	if r := repair(s); r != "" {
		return []string{r}
	}
	return nil
}
