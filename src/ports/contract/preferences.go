package contract

import (
	core "github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/ports"
)

const GroupServices = "services"

type ServicesPrefs struct{ p core.Preferences }

func Services(p core.Preferences) ServicesPrefs { return ServicesPrefs{p: p} }

func (s ServicesPrefs) Get() (*ports.ServicesGroup, error) {
	// invariant: nil when the owner chose nothing, so every default holds.
	var g *ports.ServicesGroup
	err := s.p.Read(GroupServices, &g)
	return g, err
}

func (s ServicesPrefs) Set(g *ports.ServicesGroup) error { return s.p.Write(GroupServices, g) }
