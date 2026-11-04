package machines

import (
	"context"

	model "github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

type Edited = model.Edited

type NothingToChangeError struct{ Name string }

func (e *NothingToChangeError) Error() string { return "say what to change on " + e.Name }

type DiskShrinksError struct {
	Name string
	Has  int64
}

func (e *DiskShrinksError) Error() string { return "a VM's disk only grows" }

func (s *Service) CheckEdit(name string, r Resources) error {
	if _, err := s.VM(name, "edit"); err != nil {
		return err
	}
	if r == (Resources{}) {
		return &NothingToChangeError{Name: name}
	}
	return nil
}

// invariant: an edit writes the sizes the VM boots with; a running VM keeps
// its own until its next start, and a disk is never shrunk.
func (s *Service) Edit(ctx context.Context, name string, r Resources) (Edited, error) {
	if err := s.CheckEdit(name, r); err != nil {
		return Edited{}, err
	}
	rec, err := s.VM(name, "edit")
	if err != nil {
		return Edited{}, err
	}
	if r.DiskBytes != 0 && r.DiskBytes < rec.Resources.DiskBytes {
		return Edited{}, &DiskShrinksError{Name: name, Has: rec.Resources.DiskBytes}
	}
	if err := s.EngineUp(ctx); err != nil {
		return Edited{}, err
	}
	size := rec.Resources
	if r.CPUs != 0 {
		size.CPUs = r.CPUs
	}
	if r.MemoryBytes != 0 {
		size.MemoryBytes = r.MemoryBytes
	}
	if r.DiskBytes != 0 {
		size.DiskBytes = r.DiskBytes
	}
	if err := s.engine.Resize(ctx, name, model.Size{CPUs: size.CPUs, Memory: size.MemoryBytes, Disk: size.DiskBytes}); err != nil {
		return Edited{}, err
	}
	rec.Resources = size
	if err := s.writeRecord(rec); err != nil {
		return Edited{}, err
	}
	return Edited{Name: name, Changed: r, Running: s.engineState(ctx, name) == contract.DiskRunning}, nil
}
