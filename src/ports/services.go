package ports

import "time"

type Phase string

const (
	PhaseNone      Phase = ""
	PhasePublished Phase = "published"
	PhaseClosing   Phase = "closing"
	PhaseWaiting   Phase = "waiting"
	PhaseRefused   Phase = "refused"
	PhaseGone      Phase = "gone"
	PhaseUnknown   Phase = "unknown"
)

type URLSet struct {
	Local  string `json:"local"`
	Public string `json:"public"`
}

type Target struct {
	VM   string
	Port int
}

type ServiceView struct {
	VM        string `json:"vm"`
	GuestPort int    `json:"guest_port"`
	HostPort  int    `json:"host_port"`
	State     Phase  `json:"state"`
	Process   string `json:"process"`
	HTTP      bool   `json:"http"`
	Pinned    bool   `json:"pinned"`
	Profile   string `json:"profile"`
	URLs      URLSet `json:"urls"`
	// value: the address it listens on inside its VM, as its guest reports it.
	Bind       string     `json:"bind"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	Detail     string     `json:"detail"`
}

type EngineInfo struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
	// value: the agent's plist is on this host: a daemon not running is
	// stopped, not missing (R9.D3); never in the services/v1 contract.
	Installed bool `json:"-"`
}

type HostServices struct {
	Name       string        `json:"name"`
	Here       bool          `json:"here"`
	Reachable  bool          `json:"reachable"`
	Engine     EngineInfo    `json:"daemon"`
	PublicHost string        `json:"public_host"`
	Serve      string        `json:"serve"`
	HTTPS      string        `json:"https"`
	Range      *Range        `json:"range,omitempty"`
	Services   []ServiceView `json:"services"`
	// value: what listens on each VM of this Mac, as its engine heard it.
	Listening []Machine `json:"listening,omitempty"`
	// value: every port this Mac and its VMs listen on, in its two classes.
	Ports []Port `json:"ports,omitempty"`
}

type Range struct {
	Low      int `json:"low"`
	High     int `json:"high"`
	Leased   int `json:"leased"`
	Free     int `json:"free"`
	MaxPerVM int `json:"max_per_vm"`
}

type AllMachines struct {
	Schema string         `json:"schema"`
	Hosts  []HostServices `json:"hosts"`
}

type Listing struct {
	AllMachines AllMachines `json:"fleet"`
	// value: this Mac's node on the tailnet, zero inside a VM: its Mac's
	// sharing is said by each of its rows.
	Tailnet Status `json:"tailnet"`
}
