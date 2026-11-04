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

func (s *Service) CheckEdit(name string, c model.Change) error {
	rec, err := s.VM(name, "edit")
	if err != nil {
		return err
	}
	switch {
	case c.None():
		return &NothingToChangeError{Name: name}
	case (c.Check != "" || c.NoCheck) && rec.Lifetime != AlwaysOn:
		return &KindRuleError{Why: errCheckForServers.Why, Fix: "a workspace's health is three steps: station machines " + name}
	case c.Check != "" && c.NoCheck:
		return &KindRuleError{Why: "--check and --no-check ask opposite things", Fix: "station machines edit " + name + " --check <url>"}
	case c.Check != "" && !validCheck(c.Check):
		return &BadCheckError{Value: c.Check}
	}
	return nil
}

func (s *Service) Edit(ctx context.Context, name string, c model.Change) (Edited, error) {
	// invariant: an edit writes the sizes the VM boots with; a running VM keeps
	// its own until its next start, and a disk is never shrunk. A server's
	// check is its record's: the next health run reads it.
	r := c.Resources
	if err := s.CheckEdit(name, c); err != nil {
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
	if r != (Resources{}) {
		if err := s.engine.Resize(ctx, name, model.Size{CPUs: size.CPUs, Memory: size.MemoryBytes, Disk: size.DiskBytes}); err != nil {
			return Edited{}, err
		}
	}
	rec.Resources = size
	switch {
	case c.NoCheck:
		rec.Check = ""
	case c.Check != "":
		rec.Check = c.Check
	}
	if err := s.writeRecord(rec); err != nil {
		return Edited{}, err
	}
	return Edited{Name: name, Changed: r, Check: c.Check, NoCheck: c.NoCheck, Running: s.engineState(ctx, name) == contract.DiskRunning}, nil
}
