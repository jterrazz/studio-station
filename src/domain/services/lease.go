package services

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	LeasesSchema = "station.leases/v1"
	Schema       = "station.services/v1"
)

type Lease struct {
	VM         string    `json:"vm"`
	GuestPort  int       `json:"guest_port"`
	HostPort   int       `json:"host_port"`
	Profile    string    `json:"profile,omitempty"`
	Pinned     bool      `json:"pinned"`
	HTTP       *bool     `json:"http,omitempty"`
	ReservedAt time.Time `json:"reserved_at"`
	LastSeenAt time.Time `json:"last_seen_at,omitzero"`
}

func (l Lease) Key() string { return l.VM + ":" + strconv.Itoa(l.GuestPort) }

type leaseFile struct {
	Schema string  `json:"schema"`
	Leases []Lease `json:"leases"`
}

type Store struct {
	fs   system.FS
	path string
}

func NewStore(files system.FS, state string) Store {
	return Store{fs: files, path: path.Join(state, "leases.json")}
}

func (s Store) Load() ([]Lease, error) {
	data, err := s.fs.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f leaseFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Leases, nil
}

func (s Store) Save(leases []Lease) error {
	// invariant: the store is written whole to a temporary file then renamed, so a
	// reader (the CLI while the daemon writes) never sees half a file.
	sort.Slice(leases, func(i, j int) bool {
		if leases[i].VM != leases[j].VM {
			return leases[i].VM < leases[j].VM
		}
		return leases[i].GuestPort < leases[j].GuestPort
	})
	if leases == nil {
		leases = []Lease{}
	}
	data, err := json.MarshalIndent(leaseFile{Schema: LeasesSchema, Leases: leases}, "", "  ")
	if err != nil {
		return err
	}
	if err := s.fs.MkdirAll(path.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := s.fs.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return s.fs.Rename(tmp, s.path)
}

func Find(leases []Lease, vm string, port int) (found Lease, index int) {
	for i, l := range leases {
		if l.VM == vm && l.GuestPort == port {
			return l, i
		}
	}
	return Lease{}, -1
}

func Without(leases []Lease, keep func(Lease) bool) (kept, gone []Lease) {
	for _, l := range leases {
		if keep(l) {
			kept = append(kept, l)
		} else {
			gone = append(gone, l)
		}
	}
	return kept, gone
}

func Allocate(p Policy, leases []Lease, vm string, port int, busy func(int) bool) (Lease, error) {
	// invariant: a lease is sticky on (vm, guest port): an existing one is returned
	// as it is; a new one takes the LOWEST port of the range no lease holds and the
	// host does not hold either.
	if l, i := Find(leases, vm, port); i >= 0 {
		return l, nil
	}
	held, mine := map[int]bool{}, 0
	for _, l := range leases {
		held[l.HostPort] = true
		if l.VM == vm {
			mine++
		}
	}
	if mine >= p.MaxPerVM {
		return Lease{}, &MaxPerVMError{VM: vm, Max: p.MaxPerVM}
	}
	for hp := p.Low; hp <= p.High; hp++ {
		if held[hp] || (busy != nil && busy(hp)) {
			continue
		}
		return Lease{VM: vm, GuestPort: port, HostPort: hp}, nil
	}
	return Lease{}, &RangeFullError{Low: p.Low, High: p.High}
}
