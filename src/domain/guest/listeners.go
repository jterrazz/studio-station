package guest

import (
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
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
	inode   string
}

type Event struct {
	Listen   *Listener
	Unlisten int
}

func ParseProcNet(tcp, tcp6 []byte) []Listener {
	// invariant: /proc/net/tcp's local address is the address in host byte order
	// (little-endian on arm64): 0100007F is 127.0.0.1; an IPv6 address is four such
	// 32-bit words; only rows whose state is 0A (LISTEN) count.
	byPort := map[int]Listener{}
	for _, table := range [][]byte{tcp6, tcp} {
		for _, line := range strings.Split(string(table), "\n")[1:] {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" {
				continue
			}
			hexIP, hexPort, ok := strings.Cut(f[1], ":")
			if !ok {
				continue
			}
			port, err := strconv.ParseUint(hexPort, 16, 16)
			if err != nil {
				continue
			}
			ip, ok := decodeIP(hexIP)
			if !ok {
				continue
			}
			byPort[int(port)] = Listener{Port: int(port), IP: ip, Source: SourceSocket, inode: f[9]}
		}
	}
	return sorted(byPort)
}

func decodeIP(h string) (string, bool) {
	raw, err := hex.DecodeString(h)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return "", false
	}
	out := make([]byte, len(raw))
	for w := 0; w < len(raw); w += 4 {
		for i := range 4 {
			out[w+i] = raw[w+3-i]
		}
	}
	if len(out) == 16 && allZero(out[:10]) && out[10] == 0xff && out[11] == 0xff {
		out = out[12:]
	}
	if len(out) == 4 {
		parts := make([]string, 4)
		for i, b := range out {
			parts[i] = strconv.Itoa(int(b))
		}
		return strings.Join(parts, "."), true
	}
	return formatIPv6(out), true
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func formatIPv6(b []byte) string {
	groups := make([]int, 8)
	for i := range groups {
		groups[i] = int(b[2*i])<<8 | int(b[2*i+1])
	}
	start, length := -1, 0
	for i := 0; i < 8; {
		if groups[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && groups[j] == 0 {
			j++
		}
		if j-i > length && j-i > 1 {
			start, length = i, j-i
		}
		i = j
	}
	var parts []string
	for i := 0; i < 8; i++ {
		if i == start {
			parts = append(parts, "")
			if i == 0 {
				parts = append(parts, "")
			}
			i += length - 1
			if i == 7 {
				parts = append(parts, "")
			}
			continue
		}
		parts = append(parts, strconv.FormatInt(int64(groups[i]), 16))
	}
	return strings.Join(parts, ":")
}

func sorted(byPort map[int]Listener) []Listener {
	out := make([]Listener, 0, len(byPort))
	for _, l := range byPort {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func ParseLsof(data []byte) []Listener {
	byPort := map[int]Listener{}
	var pid int
	var command string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			command = line[1:]
		case 'n':
			i := strings.LastIndex(line, ":")
			if i < 0 {
				continue
			}
			port, err := strconv.Atoi(line[i+1:])
			if err != nil {
				continue
			}
			ip := strings.Trim(line[1:i], "[]")
			if ip == "*" {
				ip = "0.0.0.0"
			}
			if _, seen := byPort[port]; !seen || strings.Contains(byPort[port].IP, ":") {
				byPort[port] = Listener{Port: port, IP: ip, Process: command, PID: pid, Source: SourceSocket}
			}
		}
	}
	return sorted(byPort)
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
