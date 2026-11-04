package machines

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

type recordFile struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Version     string            `json:"version"`
	Lifetime    Lifetime          `json:"lifetime"`
	Want        string            `json:"want,omitempty"`
	Network     string            `json:"network,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	CPUs        int               `json:"cpus"`
	MemoryBytes int64             `json:"memory_bytes"`
	DiskBytes   int64             `json:"disk_bytes"`
	Lent        json.RawMessage   `json:"lent,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

func (r recordFile) record() VMRecord {
	return VMRecord{
		Name: r.Name, Image: r.Image, Version: r.Version, Lifetime: r.Lifetime, Want: r.Want, Network: r.Network, Env: r.Env,
		Resources: Resources{CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes},
		Lent:      lentNames(r.Lent), CreatedAt: r.CreatedAt,
	}
}

// invariant: the lend entries are the secrets' to write: a record holds them
// as names (before lends expired) or as {name, at, until}, and machines only
// reads their names.
func lentNames(raw json.RawMessage) []string {
	var names []string
	if len(raw) == 0 || json.Unmarshal(raw, &names) == nil {
		return names
	}
	var lends []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &lends) != nil {
		return nil
	}
	for _, l := range lends {
		names = append(names, l.Name)
	}
	return names
}

func (s *Service) records() ([]VMRecord, error) {
	entries, err := s.h.FS.ReadDir(s.recordsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &RecordError{Path: s.recordsDir(), Cause: err}
	}
	var out []VMRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		file := path.Join(s.recordsDir(), e.Name())
		data, err := s.h.FS.ReadFile(file)
		if err != nil {
			return nil, &RecordError{Path: file, Cause: err}
		}
		var rec recordFile
		if err := json.Unmarshal(data, &rec); err != nil {
			return nil, &RecordError{Path: file, Cause: err}
		}
		out = append(out, rec.record())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func kindOfImage(image string) catalog.MachineKind {
	for _, i := range catalog.Images {
		if i.Name == image {
			return i.Kind
		}
	}
	return catalog.Linux
}

func (s *Service) Records() ([]VMRecord, error) { return s.records() }

func (s *Service) vms(ctx context.Context, host string, live map[string]DaemonVM) ([]VM, error) {
	// invariant: the daemon's view wins; without it, no engine or a listing that
	// fails means nothing is running: a lasting VM reads stopped, a temporary one stale.
	records, err := s.records()
	if err != nil || len(records) == 0 {
		return nil, err
	}
	engine := map[string]contract.Disk{}
	if s.engine != nil {
		if disks, listErr := s.engine.Disks(ctx); listErr == nil {
			for _, d := range disks {
				engine[d.VM] = d
			}
		}
	}
	out := make([]VM, 0, len(records))
	for _, r := range records {
		info := engine[r.Name]
		vm := VM{
			Name: r.Name, Image: r.Image, Version: r.Version, Host: host, Kind: kindOfImage(r.Image),
			Lifetime: r.Lifetime, State: stateOf(r.Lifetime, info), Resources: r.Resources, SizeGB: int(info.Bytes / 1e9),
			Network: networkOf(r), Guest: "unknown", Lent: r.Lent, CreatedAt: r.CreatedAt,
		}
		if d, ok := live[r.Name]; ok && live != nil {
			if d.State != "" {
				vm.State = VMState(d.State)
			}
			vm.Guest, vm.IP, vm.Detail = d.Guest, d.IP, d.Detail
		}
		out = append(out, vm)
	}
	return out, nil
}

func networkOf(r VMRecord) string {
	if r.Network == "" {
		return NetworkNAT
	}
	return r.Network
}

func stateOf(l Lifetime, d contract.Disk) VMState {
	switch d.State {
	case contract.DiskRunning:
		return Running
	case contract.DiskSuspended:
		return Suspended
	case contract.DiskStopped, diskMissing:
	}
	if l == Temporary {
		return Stale
	}
	return Stopped
}
