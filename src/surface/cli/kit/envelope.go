package kit

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/page"
)

func (e Env) envelope(ctx context.Context) *output.Envelope {
	// invariant: the machine a document is about is the one its call ran on,
	// named as the interface names it (SPEC §2.5, §11.1); a name the fleet
	// does not hold is said as it was typed.
	if e.Now == nil {
		return nil
	}
	out := &output.Envelope{At: e.Now()}
	target := e.Target()
	if target == e.Self {
		kind := e.Kind
		if kind == "" {
			kind = machine.Mac
		}
		out.Machine = output.Machine{Name: e.Self.Name, Label: e.Self.Name, Kind: string(kind), ThisMac: kind == machine.Mac}
		if kind == machine.Mac {
			out.Machine.Label = page.MacName(e.Self.Name, e.Self.Name)
		}
		return out
	}
	out.Machine = output.Machine{Name: target.Name, Label: target.Name, Kind: string(machine.Mac)}
	list, err := call.Read(ctx, e.Calls, e.Self, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
	if err != nil {
		return out
	}
	for _, h := range list.Fleet.Hosts {
		if h.Name == target.Name || page.MacName(h.Name, e.Self.Name) == target.Name {
			out.Machine = output.Machine{Name: h.Name, Label: page.MacName(h.Name, e.Self.Name), Kind: string(machine.Mac), ThisMac: h.Here}
			return out
		}
		for _, vm := range h.VMs {
			if vm.Name == target.Name {
				mac := h.Name
				out.Machine = output.Machine{Name: vm.Name, Label: vm.Name, Kind: string(vm.Lifetime.Kind()), On: &mac}
				return out
			}
		}
	}
	return out
}
