package guest

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/ports"
)

const (
	SourceSocket = "socket"
	SourceDocker = "docker"
)

type Listener struct {
	Port    int    `json:"port"`
	IP      string `json:"ip"`
	Process string `json:"process"`
	PID     int    `json:"pid"`
	Source  string `json:"source"`
}

type Event struct {
	Listen   *Listener
	Unlisten int
}

func Sockets(ls []ports.Listener) []Listener {
	// invariant: one listener per port: an IPv4 address wins its port over an
	// IPv6 one, and a later address of the same family over an earlier one.
	byPort := map[int]Listener{}
	for _, l := range ls {
		if held, seen := byPort[l.Port]; seen && !strings.Contains(held.IP, ":") && l.IPv6() {
			continue
		}
		byPort[l.Port] = Listener{Port: l.Port, IP: l.Bind, Process: l.Process, PID: l.PID, Source: SourceSocket}
	}
	return sorted(byPort)
}

func sorted(byPort map[int]Listener) []Listener {
	out := make([]Listener, 0, len(byPort))
	for _, l := range byPort {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

type dockerRow struct {
	Names string `json:"Names"`
	Ports string `json:"Ports"`
}

func ParseDockerPS(data []byte) []Listener {
	byPort := map[int]Listener{}
	for _, line := range strings.Split(string(data), "\n") {
		var row dockerRow
		if strings.TrimSpace(line) == "" || json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		for _, entry := range strings.Split(row.Ports, ",") {
			published, _, ok := strings.Cut(strings.TrimSpace(entry), "->")
			if !ok {
				continue
			}
			i := strings.LastIndex(published, ":")
			if i < 0 {
				continue
			}
			port, err := strconv.Atoi(published[i+1:])
			if err != nil {
				continue
			}
			ip := strings.Trim(published[:i], "[]")
			if _, seen := byPort[port]; !seen {
				byPort[port] = Listener{Port: port, IP: ip, Process: row.Names, Source: SourceDocker}
			}
		}
	}
	for port, l := range byPort {
		if l.IP == "::" {
			l.IP = "0.0.0.0"
			byPort[port] = l
		}
	}
	return sorted(byPort)
}

func Merge(sockets, docker []Listener) []Listener {
	byPort := map[int]Listener{}
	for _, l := range sockets {
		byPort[l.Port] = l
	}
	for _, l := range docker {
		byPort[l.Port] = l
	}
	return sorted(byPort)
}

func Filter(ls []Listener, minPort int, ignored []int) []Listener {
	out := ls[:0:0]
	for _, l := range ls {
		skip := l.Port < minPort
		for _, p := range ignored {
			skip = skip || p == l.Port
		}
		if !skip {
			out = append(out, l)
		}
	}
	return out
}

func Diff(before, after []Listener) []Event {
	was := map[int]Listener{}
	for _, l := range before {
		was[l.Port] = l
	}
	var out []Event
	now := map[int]bool{}
	for _, l := range after {
		now[l.Port] = true
		if old, ok := was[l.Port]; !ok || old.IP != l.IP || old.Process != l.Process || old.Source != l.Source {
			listen := l
			out = append(out, Event{Listen: &listen})
		}
	}
	for _, l := range before {
		if !now[l.Port] {
			out = append(out, Event{Unlisten: l.Port})
		}
	}
	return out
}
