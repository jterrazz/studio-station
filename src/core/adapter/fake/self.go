package fake

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/machine"
)

type Self struct{ p machine.Profile }

func NewSelf(p machine.Profile) *Self {
	// value: the machine a spec stands on, by its profile; its capabilities
	// are the other contracts', so Self drops them.
	return &Self{p: p}
}

func (s *Self) Read(context.Context) (machine.Ref, machine.Kind, machine.System, machine.Hardware, error) {
	return s.p.Ref, s.p.Kind, s.p.System, s.p.Hardware, nil
}
