package fake

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

var ErrNoAnswer = errors.New("the check did not answer")

type HealthProbe struct{ w *world.World }

var _ contract.HealthProbe = (*HealthProbe)(nil)

func HealthProbeOf(w *world.World) *HealthProbe { return &HealthProbe{w: w} }

func (p *HealthProbe) Probe(_ context.Context, t machines.CheckTarget) error {
	// invariant: a check of the world answers while the VM its name holds
	// runs, until a scripted fault of HealthProbe.Probe stands for what
	// inside stopped answering it; a check on any other name answers.
	p.w.Lock()
	defer p.w.Unlock()
	if p.w.Fails("HealthProbe.Probe") {
		return ErrNoAnswer
	}
	if mac := p.w.Holder(t.Name); mac != "" && !p.w.Macs[mac].VMs[t.Name].Running() {
		return ErrNoAnswer
	}
	return nil
}
