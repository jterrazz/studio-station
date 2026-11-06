package machines

import (
	"time"

	"github.com/jterrazz/studio-station/src/core/finding"
)

const ListSchema = "station.machines.list/v1"

type ListDoc struct {
	Schema   string         `json:"schema"`
	Macs     []ListMac      `json:"macs"`
	Findings []finding.JSON `json:"findings"`
}

type ListMac struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	ThisMac   bool       `json:"this_mac"`
	Reachable bool       `json:"reachable"`
	Answered  time.Time  `json:"last_answer,omitzero"`
	IP        string     `json:"ip"`
	User      string     `json:"user"`
	Engine    ListEngine `json:"engine"`
	Station   string     `json:"station_version"`
	VMs       []ListVM   `json:"vms"`
}

type ListEngine struct {
	State   string `json:"state"`
	Version string `json:"version"`
}

type ListVM struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Lifetime string `json:"lifetime"`
	State    string `json:"state"`
	Wanted   string `json:"wanted"`
	// value: SPEC §7.2's steps a workspace has, each passed or not.
	Health   ListHealth   `json:"health"`
	Restarts ListRestarts `json:"restarts"`
	IP       string       `json:"ip"`
	// value: on its Mac's network; false is behind its Mac.
	Bridged bool      `json:"bridged"`
	Size    ListSize  `json:"size"`
	Image   ListImage `json:"image"`
	Detail  string    `json:"detail,omitempty"`
	Created time.Time `json:"created_at"`
}

type ListHealth struct {
	Running bool `json:"running"`
	Address bool `json:"address"`
	SSH     bool `json:"ssh"`
}

type ListSize struct {
	CPUs        int   `json:"cpus"`
	MemoryBytes int64 `json:"memory_bytes"`
	DiskBytes   int64 `json:"disk_bytes"`
}

type ListImage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListRestarts struct {
	Count   int        `json:"count"`
	LastAt  *time.Time `json:"last_at"`
	Looping bool       `json:"looping"`
	WaitS   int64      `json:"wait_s"`
}

func restartsOf(r Restarts) ListRestarts {
	out := ListRestarts{Count: r.Count, Looping: r.Looping, WaitS: int64(r.Wait.Seconds())}
	if !r.Last.IsZero() {
		last := r.Last.UTC()
		out.LastAt = &last
	}
	return out
}

func ToList(f AllMachines) ListDoc {
	out := ListDoc{Schema: ListSchema, Macs: make([]ListMac, 0, len(f.Macs)), Findings: finding.AllMachinesDoc(Findings(f))}
	for _, h := range f.Macs {
		m := ListMac{
			Name: h.Name, Label: Label(f, h), ThisMac: h.ThisMac, Reachable: h.Reachable, Answered: h.Answered, IP: h.Node.IP, User: h.User,
			Engine: ListEngine{State: engineState(h), Version: h.Engine.Version}, Station: h.StationVersion, VMs: []ListVM{},
		}
		for _, vm := range Ordered(h.VMs) {
			m.VMs = append(m.VMs, ListVM{
				Name: vm.Name, Kind: string(vm.Lifetime.Kind()), Lifetime: string(vm.Lifetime), State: string(vm.State),
				Wanted: Wanted(vm), Health: HealthOf(vm), Restarts: restartsOf(vm.Restarts), IP: vm.IP, Bridged: vm.Network == "bridged",
				Size:  ListSize{CPUs: vm.Resources.CPUs, MemoryBytes: vm.Resources.MemoryBytes, DiskBytes: vm.Resources.DiskBytes},
				Image: ListImage{Name: vm.Image, Version: vm.Version}, Detail: vm.Detail, Created: vm.CreatedAt,
			})
		}
		out.Macs = append(out.Macs, m)
	}
	return out
}

func engineState(h MacView) string {
	switch {
	case !h.Reachable:
		return "unknown"
	case !h.Engine.Known:
		return "none"
	case h.Engine.Running:
		return "running"
	case !h.Engine.Installed && h.ThisMac:
		return "not-set-up"
	}
	return "stopped"
}

func Wanted(vm VM) string {
	if vm.Want == "running" {
		return "running"
	}
	return "stopped"
}

func HealthOf(vm VM) ListHealth {
	// value: a VM that does not run has passed no step.
	running := vm.State == Running
	return ListHealth{Running: running, Address: running && vm.IP != "", SSH: running && (vm.Guest == "connected" || vm.SSH)}
}
