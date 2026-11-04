package machines

import (
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

const FleetSchema = "station.machines/v1"

type WireNode struct {
	Online bool   `json:"online"`
	IP     string `json:"ip"`
}

type WireVM struct {
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Image       string    `json:"image"`
	Version     string    `json:"version"`
	Lifetime    string    `json:"lifetime"`
	State       string    `json:"state"`
	CPUs        int       `json:"cpus"`
	MemoryBytes int64     `json:"memory_bytes"`
	DiskBytes   int64     `json:"disk_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	IP          string    `json:"ip"`
	Network     string    `json:"network"`
	Guest       string    `json:"guest"`
	Detail      string    `json:"detail,omitempty"`
	Lent        []string  `json:"lent"`
}

type WireDaemon struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
}

type WireHost struct {
	Name           string      `json:"name"`
	Here           bool        `json:"here"`
	Reachable      bool        `json:"reachable"`
	StationVersion string      `json:"station_version"`
	User           string      `json:"user"`
	Node           WireNode    `json:"node"`
	Daemon         *WireDaemon `json:"daemon,omitempty"`
	VMs            []WireVM    `json:"vms"`
}

type WireFleet struct {
	Schema string     `json:"schema"`
	Hosts  []WireHost `json:"hosts"`
}

func ToWire(f Fleet) WireFleet {
	// invariant: the wire types ARE the `station.machines/v1` document; the
	// presenter encodes them and the fan-out decodes each other host's answer, so
	// the schema has one owner.
	out := WireFleet{Schema: FleetSchema, Hosts: make([]WireHost, 0, len(f.Hosts))}
	for _, h := range f.Hosts {
		w := WireHost{
			Name: h.Name, Here: h.Here, Reachable: h.Reachable, StationVersion: h.StationVersion, User: h.User,
			Node: toWire(h.Node), VMs: make([]WireVM, 0, len(h.VMs)),
		}
		if h.Daemon.Known {
			w.Daemon = &WireDaemon{Running: h.Daemon.Running, Version: h.Daemon.Version}
		}
		for _, vm := range h.VMs {
			lent := vm.Lent
			if lent == nil {
				lent = []string{}
			}
			w.VMs = append(w.VMs, WireVM{
				Name: vm.Name, Kind: string(vm.Kind), Image: vm.Image, Version: vm.Version,
				Lifetime: string(vm.Lifetime), State: string(vm.State), CPUs: vm.Resources.CPUs,
				MemoryBytes: vm.Resources.MemoryBytes, DiskBytes: vm.Resources.DiskBytes,
				CreatedAt: vm.CreatedAt, IP: vm.IP, Network: vm.Network, Guest: vm.Guest, Detail: vm.Detail, Lent: lent,
			})
		}
		out.Hosts = append(out.Hosts, w)
	}
	return out
}

func (w WireHost) View(name, user string) HostView {
	view := HostView{Name: name, User: user, StationVersion: w.StationVersion, Reachable: true, Node: w.Node.node()}
	if w.Daemon != nil {
		view.Daemon = EngineView{Known: true, Running: w.Daemon.Running, Version: w.Daemon.Version}
	}
	for _, vm := range w.VMs {
		view.VMs = append(view.VMs, VM{
			Name: vm.Name, Image: vm.Image, Version: vm.Version, Host: name, Kind: catalog.MachineKind(vm.Kind),
			Lifetime: Lifetime(vm.Lifetime), State: VMState(vm.State),
			Resources: Resources{CPUs: vm.CPUs, MemoryBytes: vm.MemoryBytes, DiskBytes: vm.DiskBytes},
			IP:        vm.IP, Network: vm.Network, Guest: vm.Guest, Detail: vm.Detail, Lent: vm.Lent, CreatedAt: vm.CreatedAt,
		})
	}
	return view
}

func toWire(n Node) WireNode { return WireNode(n) }

func (w WireNode) node() Node { return Node(w) }
