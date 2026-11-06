package contract

import (
	core "github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/ports"
)

const GroupPorts = "ports"

const (
	// value: alias of the key station v2 kept the ports policy under, read
	// under ports and written there at the next write.
	RetiredGroupPorts = "services"
)

type PortsPrefs struct{ p core.Preferences }

func Ports(p core.Preferences) PortsPrefs { return PortsPrefs{p: p} }

func (s PortsPrefs) Get() (*ports.PortsGroup, error) {
	// invariant: nil when the owner chose nothing, so every default holds.
	var g *ports.PortsGroup
	err := s.p.Read(GroupPorts, &g)
	return g, err
}

func (s PortsPrefs) Set(g *ports.PortsGroup) error { return s.p.Write(GroupPorts, g) }
