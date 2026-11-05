package kit

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
)

type Held struct {
	// value: a VM of the fleet, the Mac that holds it, and whether it runs.
	VM, Mac   string
	MacIsHere bool
	Running   bool
	Kind      machine.Kind
}

func Holder(ctx context.Context, env Env, vm string) (Held, bool) {
	list, err := call.Read(ctx, env.Calls, env.Self, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
	if err != nil {
		return Held{}, false
	}
	for _, h := range list.Fleet.Hosts {
		for _, v := range h.VMs {
			if v.Name == vm {
				return Held{VM: vm, Mac: h.Name, MacIsHere: h.Here, Running: v.State == machines.Running, Kind: v.Lifetime.Kind()}, true
			}
		}
	}
	return Held{}, false
}

func (h Held) OnTarget(env Env) bool {
	// value: whether the Mac the command runs on is the one that holds the VM.
	t := env.Target()
	if t.Name == "" || t.Name == env.Self.Name {
		return h.MacIsHere
	}
	return t.Name == h.Mac
}
