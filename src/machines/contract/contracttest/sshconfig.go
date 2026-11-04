package contracttest

import (
	"errors"
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

type SSHConfigSubject struct {
	Config contract.SSHConfig
	// value: the names of the blocks station manages, sorted.
	Managed func(t *testing.T) []string
	// value: the owner writes a Host of their own, outside station's blocks.
	Foreign func(t *testing.T, name string)
}

func block(name string) contract.HostBlock { return contract.HostBlock{Name: name, User: "u"} }

func vm(name string) contract.HostBlock {
	return contract.HostBlock{Name: name, User: "vmu", Behind: Mac, Here: true}
}

func SSHConfig(t *testing.T, subject func(t *testing.T) SSHConfigSubject) {
	t.Run("a block put is station's, a block removed no longer", func(t *testing.T) {
		s := subject(t)
		if err := s.Config.Put(t.Context(), []contract.HostBlock{block(Mac), vm(VM)}); err != nil {
			t.Fatal(err)
		}
		if got := s.Managed(t); !slices.Equal(got, []string{VM, Mac}) {
			t.Fatalf("after the put station manages %v, want [%s %s]", got, VM, Mac)
		}
		if err := s.Config.Remove(t.Context(), VM); err != nil {
			t.Fatal(err)
		}
		if got := s.Managed(t); !slices.Equal(got, []string{Mac}) {
			t.Errorf("after the remove station manages %v, want [%s]", got, Mac)
		}
	})
	t.Run("a block put again stays one block", func(t *testing.T) {
		s := subject(t)
		for range 2 {
			if err := s.Config.Put(t.Context(), []contract.HostBlock{block(Mac)}); err != nil {
				t.Fatal(err)
			}
		}
		if got := s.Managed(t); !slices.Equal(got, []string{Mac}) {
			t.Errorf("station manages %v, want one [%s]", got, Mac)
		}
	})
	t.Run("the whole set replaces the blocks: one not in it goes", func(t *testing.T) {
		s := subject(t)
		if err := s.Config.Put(t.Context(), []contract.HostBlock{block(Mac), vm(VM), vm(Renamed)}); err != nil {
			t.Fatal(err)
		}
		if err := s.Config.Replace(t.Context(), []contract.HostBlock{block(Mac), vm(VM)}); err != nil {
			t.Fatal(err)
		}
		if got := s.Managed(t); !slices.Equal(got, []string{VM, Mac}) {
			t.Errorf("after the replace station manages %v, want [%s %s]", got, VM, Mac)
		}
	})
	t.Run("a Host the owner wrote is never shadowed, and the other blocks are still written", func(t *testing.T) {
		s := subject(t)
		s.Foreign(t, Mac)
		var refused *contract.SSHConfigError
		if err := s.Config.Put(t.Context(), []contract.HostBlock{block(Mac), vm(VM)}); !errors.As(err, &refused) {
			t.Fatalf("shadowing the owner's Host returned %v, want an SSHConfigError", err)
		}
		if got := s.Managed(t); !slices.Equal(got, []string{VM}) {
			t.Errorf("station manages %v, want only [%s]", got, VM)
		}
	})
}
