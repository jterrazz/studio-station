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
	// value: on a Mac that does not answer: as it was at its last answer,
	// its state unknown.
	UnknownState VMState = "unknown"
)

type VM struct {
	Name, Image, Version, Host string
	Kind                       catalog.MachineKind
	Lifetime                   Lifetime
	State                      VMState
	Resources                  Resources
	SizeGB                     int
	IP, Network, Guest, Detail string
	// value: the state its record asks for, running or stopped.
	Want      string
	CreatedAt time.Time
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
	// value: when this Mac last had its answer; zero for one never heard.
	Answered time.Time
	Node     Node
	Daemon   EngineView
	VMs      []VM
}

type Fleet struct {
	Hosts []HostView
	// value: when it was read.
	Read time.Time
}

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

// value: SPEC §7.2's health steps, in order, a workspace's three.
const (
	StepRunning = "Running"
	StepAddress = "Has an address"
	StepSSH     = "SSH answers"
)

type DeleteReport struct {
	Name     string
	Warnings []string
}

type RestartEvent struct{ Phase, Detail string }

type RestartReport struct {
	Name string
	// value: a VM was restarted, its health steps passed; IP is its address.
	VM     bool
	IP     string
	Waited time.Duration
}

type Edited struct {
	Name string
	// value: the sizes the edit asked, the others zero.
	Changed Resources
	// value: the VM runs, so the change waits for its next start.
	Running bool
}

func FindVM(f Fleet, name string) (HostView, VM, bool) {
	for _, h := range f.Hosts {
		for _, vm := range h.VMs {
			if vm.Name == name {
				return h, vm, true
			}
		}
	}
	return HostView{}, VM{}, false
}
