package fake

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

var ErrNoAnswer = errors.New("the check did not answer")

type CheckRunner struct{ w *world.World }

var _ contract.CheckRunner = (*CheckRunner)(nil)

func CheckRunnerOf(w *world.World) *CheckRunner { return &CheckRunner{w: w} }

func (p *CheckRunner) Run(_ context.Context, t machines.CheckTarget) (string, error) {
	// invariant: a check of the world answers while the VM its name holds
	// runs, until a scripted fault of CheckRunner.Run stands for what
	// inside stopped answering it; a check on any other name answers. A URL
	// answers 200.
	p.w.Lock()
	defer p.w.Unlock()
	if p.w.Fails("CheckRunner.Run") {
		return "", ErrNoAnswer
	}
	if mac := p.w.Holder(t.Name); mac != "" && !p.w.Macs[mac].VMs[t.Name].Running() {
		return "", ErrNoAnswer
	}
	if t.URL == "" {
		return "", nil
	}
	return "200", nil
}
