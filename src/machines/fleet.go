package machines

import (
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

const SuperviseRetired = "supervise is retired: the engine supervises VMs (station setup)"

type Node struct {
	Online bool
	IP     string
}

type Reserved struct {
	GuestPort, HostPort int
	Profile, URL        string
	PublicHost          string
	ServeOff            bool
}

type Resources struct {
	CPUs        int
	MemoryBytes int64
	DiskBytes   int64
}

func (r Resources) Size() Size {
	return Size{CPUs: r.CPUs, Memory: r.MemoryBytes, Disk: r.DiskBytes}
}

type VMState string

const (
	Running   VMState = "running"
	Stopped   VMState = "stopped"
	Suspended VMState = "suspended"
	Stale     VMState = "stale"
	Broken    VMState = "broken"
)

type VM struct {
	Name, Image, Version, Host string
	Kind                       catalog.MachineKind
	Lifetime                   Lifetime
	State                      VMState
	Resources                  Resources
	SizeGB                     int
	IP, Network, Guest, Detail string
	Lent                       []string
	CreatedAt                  time.Time
}

type EngineView struct {
	Known, Running bool
	// value: the agent's plist is on this host: a daemon not running is
	// stopped, not missing (R9.D3); unknown for another host.
	Installed bool
	Version   string
}

type HostView struct {
	Name, User, StationVersion string
	Here, Reachable            bool
	Node                       Node
	Daemon                     EngineView
	VMs                        []VM
}

type Fleet struct{ Hosts []HostView }

type Check struct {
	Name          string
	State         catalog.State
	Value, Detail string
}

type ProbeReport struct {
	Name, User, IP string
	Checks         []Check
	Console        string
}

type CreateRequest struct {
	Image, Name string
	Lifetime    Lifetime
	Network     string
	Resources   Resources
}

type Created struct {
	VM       VM
	Services []Reserved
	Env      map[string]string
}

type Step struct{ Label string }

type DeleteReport struct {
	Name     string
	Warnings []string
}

type RestartEvent struct{ Phase, Detail string }

type RestartReport struct {
	Name   string
	Waited time.Duration
}

type Edited struct {
	Name string
	// value: the sizes the edit asked, the others zero.
	Changed Resources
	// value: the VM runs, so the change waits for its next start.
	Running bool
}
