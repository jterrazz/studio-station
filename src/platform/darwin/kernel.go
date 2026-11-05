package darwin

import "time"

type Ticks struct {
	User   uint64 `json:"user"`
	System uint64 `json:"system"`
	Idle   uint64 `json:"idle"`
	Nice   uint64 `json:"nice"`
}

type Pages struct {
	// value: host_statistics64's page counts, sized by Size bytes each.
	Size       uint64 `json:"size"`
	Internal   uint64 `json:"internal"`
	Purgeable  uint64 `json:"purgeable"`
	Wired      uint64 `json:"wired"`
	Compressor uint64 `json:"compressor"`
}

type BlockDriver struct {
	// value: an IOBlockStorageDriver and its device's Protocol Characteristics.
	Media        string `json:"media"`
	Interconnect string `json:"interconnect"`
	Location     string `json:"location"`
	Read         uint64 `json:"read"`
	Written      uint64 `json:"written"`
}

type Link struct {
	// value: an interface of NET_RT_IFLIST2 and its if_data64 byte counters.
	Name string `json:"name"`
	In   uint64 `json:"in"`
	Out  uint64 `json:"out"`
}

type Counters struct {
	CPUs     []Ticks       `json:"cpus"`
	Load     [3]float64    `json:"load"`
	Pages    Pages         `json:"pages"`
	SwapUsed uint64        `json:"swap_used"`
	Pressure int           `json:"pressure"`
	Memsize  uint64        `json:"memsize"`
	Up       time.Duration `json:"up"`
	Disks    []BlockDriver `json:"disks"`
	Links    []Link        `json:"links"`
}

type Proc struct {
	PID      int           `json:"pid"`
	Name     string        `json:"name"`
	CPU      time.Duration `json:"cpu"`
	Resident uint64        `json:"resident"`
	// value: when the process started; read for one process by its pid,
	// zero in a whole table.
	Started time.Time `json:"started,omitzero"`
}

type Procs struct {
	// value: the time of the reading on the kernel's own clock, so two
	// readings tell the time between them without the wall clock.
	At   time.Duration `json:"at"`
	List []Proc        `json:"list"`
}
