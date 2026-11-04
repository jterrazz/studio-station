package machines

import "time"

const ListSchema = "station.machines.list/v1"

type ListDoc struct {
	Schema   string        `json:"schema"`
	Macs     []ListMac     `json:"macs"`
	Findings []ListFinding `json:"findings"`
}

type ListMac struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	ThisMac   bool       `json:"this_mac"`
	Reachable bool       `json:"reachable"`
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
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Lifetime string    `json:"lifetime"`
	State    string    `json:"state"`
	IP       string    `json:"ip"`
	Network  string    `json:"network"`
	Guest    string    `json:"guest"`
	Size     ListSize  `json:"size"`
	Image    ListImage `json:"image"`
	Detail   string    `json:"detail,omitempty"`
	Lent     []string  `json:"lent"`
	Created  time.Time `json:"created_at"`
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

type ListFinding struct {
	Severity string `json:"severity"`
	Section  string `json:"section"`
	Subject  string `json:"subject"`
	Text     string `json:"text"`
	Fix      string `json:"fix,omitempty"`
}

func ToList(f Fleet) ListDoc {
	out := ListDoc{Schema: ListSchema, Macs: make([]ListMac, 0, len(f.Hosts)), Findings: []ListFinding{}}
	for _, h := range f.Hosts {
		m := ListMac{
			Name: h.Name, Label: Label(f, h), ThisMac: h.Here, Reachable: h.Reachable, IP: h.Node.IP, User: h.User,
			Engine: ListEngine{State: engineState(h), Version: h.Daemon.Version}, Station: h.StationVersion, VMs: []ListVM{},
		}
		for _, vm := range Ordered(h.VMs) {
			lent := vm.Lent
			if lent == nil {
				lent = []string{}
			}
			m.VMs = append(m.VMs, ListVM{
				Name: vm.Name, Kind: "workspace", Lifetime: string(vm.Lifetime), State: string(vm.State),
				IP: vm.IP, Network: vm.Network, Guest: vm.Guest,
				Size:  ListSize{CPUs: vm.Resources.CPUs, MemoryBytes: vm.Resources.MemoryBytes, DiskBytes: vm.Resources.DiskBytes},
				Image: ListImage{Name: vm.Image, Version: vm.Version}, Detail: vm.Detail, Lent: lent, Created: vm.CreatedAt,
			})
		}
		out.Macs = append(out.Macs, m)
	}
	for _, fd := range Findings(f) {
		out.Findings = append(out.Findings, ListFinding{Severity: string(fd.Severity), Section: string(fd.Section), Subject: fd.Subject, Text: fd.Text, Fix: fd.Fix.Command + fd.Fix.Path})
	}
	return out
}

func engineState(h HostView) string {
	switch {
	case !h.Reachable:
		return "unknown"
	case !h.Daemon.Known:
		return "none"
	case h.Daemon.Running:
		return "running"
	case !h.Daemon.Installed && h.Here:
		return "not-set-up"
	}
	return "stopped"
}
