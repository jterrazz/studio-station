package ports

import (
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/ports"
)

type PortJSON struct {
	Machine  string   `json:"machine"`
	Port     int      `json:"port"`
	Proto    string   `json:"proto"`
	Process  string   `json:"process"`
	Bind     []string `json:"bind"`
	Class    string   `json:"class"`
	Declared *string  `json:"declared"`
	State    string   `json:"state"`
	Reserved int      `json:"reserved,omitempty"`
	URL      string   `json:"url,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

type RangeJSON struct {
	Mac      string `json:"mac"`
	From     int    `json:"from"`
	To       int    `json:"to"`
	Reserved int    `json:"reserved"`
	Free     int    `json:"free"`
}

type NodeJSON struct {
	State   string `json:"state"`
	Name    string `json:"name"`
	DNSName string `json:"dns_name"`
	IP      string `json:"ip"`
	Serve   string `json:"serve"`
	Variant string `json:"variant"`
	// value: the client station found, by its absolute path.
	CLI string `json:"cli"`
}

type BodyJSON struct {
	Schema    string      `json:"schema"`
	Findings  []string    `json:"findings"`
	Ports     []PortJSON  `json:"ports"`
	LocalOnly int         `json:"local_only"`
	Range     []RangeJSON `json:"range"`
}

type ListJSON map[string]any // value: station.ports.list/v1, its node under the key of the tailnet's product, a word the catalogue owns.

// value: the words of a port's state in station.ports.list/v1.
func stateWord(s ports.ServiceView) string {
	switch {
	case s.State == ports.PhaseRefused || s.HostPort == 0:
		return "waiting"
	case s.State == ports.PhaseUnknown:
		return "engine-stopped"
	case s.State == ports.PhaseWaiting:
		return "not-listening"
	case s.Detail != "":
		return "local-only"
	}
	return "shared"
}

func PortObject(s ports.ServiceView) PortJSON {
	bind := []string{}
	if s.Bind != "" {
		bind = append(bind, s.Bind)
	}
	proto := "tcp"
	if s.HTTP {
		proto = "http"
	}
	return PortJSON{Machine: s.VM, Port: s.GuestPort, Proto: proto, Process: s.Process, Bind: bind, Class: "private",
		State: stateWord(s), Reserved: s.HostPort, URL: s.URLs.Public, Detail: s.Detail}
}

func ListContract(l ports.Listing) ListJSON {
	body := BodyJSON{Schema: "station.ports.list/v1", Findings: []string{}, Ports: []PortJSON{}, Range: []RangeJSON{}}
	var node *NodeJSON
	for _, h := range l.Fleet.Hosts {
		for _, s := range h.Services {
			body.Ports = append(body.Ports, PortObject(s))
		}
		if r := h.Range; r != nil {
			body.Range = append(body.Range, RangeJSON{Mac: h.Name, From: r.Low, To: r.High, Reserved: r.Leased, Free: r.Free})
		}
	}
	if t := l.Tailnet; t.State != "" {
		state := "on"
		switch {
		case t.State == catalog.StateAbsent:
			state = "not-installed"
		case !t.Connected:
			state = "signed-out"
		case !t.ServeOn:
			state = "serve-off"
		}
		serve := "off"
		if t.ServeOn {
			serve = "on"
		}
		node = &NodeJSON{State: state, Name: t.Name, DNSName: t.DNSName, IP: t.IP, Serve: serve, Variant: t.Variant, CLI: t.CLI}
	}
	return ListJSON{"schema": body.Schema, "findings": body.Findings, "ports": body.Ports, "local_only": body.LocalOnly, "range": body.Range,
		strings.ToLower(catalog.TailnetProduct): node}
}
