package machines

import "time"

type Record struct {
	// value: the declared VM as its Mac keeps it; never observed state.
	Name, Image, Version string
	Lifetime             Lifetime
	Want, Network        string
	Env                  map[string]string
	CPUs                 int
	MemoryBytes          int64
	DiskBytes            int64
	CreatedAt            time.Time
	// value: the names of the secrets shared with the VM, read only: the
	// accounts' store writes them beside the record.
	Lent []string
}

type Host struct {
	User string `json:"user,omitempty"`
	Key  string `json:"key,omitempty"`
}
