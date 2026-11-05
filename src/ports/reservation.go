package ports

import (
	"strconv"
	"time"
)

type Reservation struct {
	VM         string    `json:"vm"`
	GuestPort  int       `json:"guest_port"`
	HostPort   int       `json:"host_port"`
	Profile    string    `json:"profile,omitempty"`
	Pinned     bool      `json:"pinned"`
	HTTP       *bool     `json:"http,omitempty"`
	ReservedAt time.Time `json:"reserved_at"`
	LastSeenAt time.Time `json:"last_seen_at,omitzero"`
}

func (r Reservation) Key() string { return r.VM + ":" + strconv.Itoa(r.GuestPort) }

type Declaration struct {
	// value: a port of a machine declared public, and the route that reaches it.
	Machine string `json:"machine"`
	Port    int    `json:"port"`
	Via     string `json:"via"`
	// value: the setting that declared it; empty when the owner did.
	By string `json:"by,omitempty"`
}

func (d Declaration) Key() string { return d.Machine + ":" + strconv.Itoa(d.Port) }

type PortsGroup struct {
	Range        string `json:"range,omitempty"`
	MaxPerVM     int    `json:"max_per_vm,omitempty"`
	IgnoredPorts []int  `json:"ignored_ports,omitempty"`
	MinPort      int    `json:"min_port,omitempty"`
	HTTPS        string `json:"https,omitempty"`
	// value: the ports the owner stopped sharing, as <vm>:<port>.
	NotShared []string `json:"not_shared,omitempty"`
}
