package machines

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type recordFile struct {
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	Version     string    `json:"version"`
	NodeID      string    `json:"node_id,omitempty"`
	Lifetime    Lifetime  `json:"lifetime"`
	CPUs        int       `json:"cpus"`
	MemoryBytes int64     `json:"memory_bytes"`
	DiskBytes   int64     `json:"disk_bytes"`
	Lent        []string  `json:"lent,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (r recordFile) record() VMRecord {
	return VMRecord{
		Name: r.Name, Image: r.Image, Version: r.Version, NodeID: r.NodeID, Lifetime: r.Lifetime,
		Resources: system.Resources{CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes},
		Lent:      r.Lent, CreatedAt: r.CreatedAt,
	}
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

func (s *Service) vms(ctx context.Context, host string, nodes map[string]Node) ([]VM, error) {
	// invariant: no engine, or a listing that fails, means nothing is running: a
	// lasting VM reads stopped and a temporary one stale.
	records, err := s.records()
	if err != nil || len(records) == 0 {
		return nil, err
	}
	live := map[string]system.VMInfo{}
	if s.engine != nil {
		if infos, listErr := s.engine.List(ctx); listErr == nil {
			for _, i := range infos {
				live[i.Name] = i
			}
		}
	}
	out := make([]VM, 0, len(records))
	for _, r := range records {
		info := live[r.Name]
		vm := VM{
			Name: r.Name, Image: r.Image, Version: r.Version, Host: host, Kind: kindOfImage(r.Image),
			Lifetime: r.Lifetime, State: stateOf(r.Lifetime, info), Resources: r.Resources, SizeGB: info.SizeGB,
			Node: nodes[r.Name], Lent: r.Lent, CreatedAt: r.CreatedAt,
		}
		out = append(out, vm)
	}
	return out, nil
}

func stateOf(l Lifetime, info system.VMInfo) VMState {
	switch info.State {
	case system.VMRunning:
		return Running
	case system.VMSuspended:
		return Suspended
	case system.VMStopped, system.VMMissing:
	}
	if l == Temporary {
		return Stale
	}
	return Stopped
}
