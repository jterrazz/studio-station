package tailnet

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/present"
)

const joinDocs = "docs/reference/machines.md#vm-join"

type peerJSON struct {
	Name   string   `json:"name"`
	IP     string   `json:"ip"`
	Online bool     `json:"online"`
	Tags   []string `json:"tags"`
}

type statusJSON struct {
	Schema       string     `json:"schema"`
	State        string     `json:"state"`
	Connected    bool       `json:"connected"`
	BackendState string     `json:"backend_state"`
	Daemon       string     `json:"daemon"`
	Name         string     `json:"name"`
	IP           string     `json:"ip"`
	KeepAwake    bool       `json:"keep_awake"`
	Mode         string     `json:"mode"`
	AuthMethod   string     `json:"auth_method"`
	VMJoin       string     `json:"vm_join"`
	Peers        []peerJSON `json:"peers"`
}

func Status(s tailnet.Status, context string) present.Answer {
	body := statusJSON{
		Schema: "station.tailnet/v1", State: string(s.State), Connected: s.Connected, BackendState: s.BackendState,
		Daemon: string(s.Daemon), Name: s.Name, IP: s.IP, KeepAwake: s.KeepAwake, Mode: string(s.Mode),
		AuthMethod: string(s.AuthMethod), VMJoin: string(s.VMJoin), Peers: make([]peerJSON, 0, len(s.Peers)),
	}
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
			{Text: strings.Join(tags, " "), Tone: present.Muted},
		}})
	}
	blocks := []present.Block{present.Table{Rows: Rows(s)}}
	if len(peers) > 0 {
		blocks = append(blocks, present.Section{Title: "Peers", Blocks: []present.Block{present.Table{Rows: peers}}})
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station tailnet",
			Context: context,
			Blocks:  blocks,
			Hints:   hints(s),
		},
		JSON: body,
	}
}

func Rows(s tailnet.Status) []present.Row {
	rows := []present.Row{{Mark: present.MarkOf(s.State), Cells: []present.Cell{
		{Text: "tailnet", Tone: present.Strong},
		{Text: daemonText(s)},
		{Text: repair(s), Tone: present.Muted},
	}}}
	if s.Name != "" {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: "node", Tone: present.Strong},
			{Text: s.Name},
			{Text: s.IP, Tone: present.Muted},
		}})
	}
	if s.Daemon == tailnet.Userspace && s.Connected {
		mark, text := present.MarkOK, "active"
		if !s.KeepAwake {
			mark, text = present.MarkDegraded, "not active"
		}
		rows = append(rows, present.Row{Mark: mark, Cells: []present.Cell{{Text: "keep awake", Tone: present.Strong}, {Text: text}}})
	}
	rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
		{Text: "login", Tone: present.Strong},
		{Text: string(s.Mode) + " · " + string(s.AuthMethod)},
	}}, vmJoinRow(s.VMJoin))
	return rows
}

func vmJoinRow(state catalog.State) present.Row {
	label := present.Cell{Text: "VM join", Tone: present.Strong}
	switch state {
	case catalog.StateOK:
		return present.Row{Mark: present.MarkOK, Cells: []present.Cell{label, {Text: "a key was minted"}}}
	case catalog.StateAbsent:
		return present.Row{Mark: present.MarkAbsent, Cells: []present.Cell{label, {Text: "not set up"}, {Text: joinDocs, Tone: present.Muted}}}
	case catalog.StateDegraded, catalog.StateBroken, catalog.StateUnknown:
	}
	return present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{label, {Text: "set up, not tried"}}}
}

func daemonText(s tailnet.Status) string {
	if s.BackendState == "" {
		return string(s.Daemon) + " daemon is not running"
	}
	return s.BackendState + " (" + string(s.Daemon) + " daemon)"
}

func repair(s tailnet.Status) string {
	if s.State == catalog.StateOK {
		return ""
	}
	return "station tailnet up"
}

func hints(s tailnet.Status) []string {
	if s.State == catalog.StateOK {
		return nil
	}
	return []string{"station tailnet up"}
}
