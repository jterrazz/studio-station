package fake

import (
	"context"
	"sync"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/awake/contract"
)

type Power struct {
	mu       sync.Mutex
	r        awake.Reading
	none     bool
	watchers []chan awake.Event
}

func NewPower(r awake.Reading) *Power { return &Power{r: r} }

func NewNoPower() *Power { return &Power{none: true} }

func (p *Power) Capabilities() contract.PowerCaps {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.none {
		return contract.PowerCaps{}
	}
	return contract.PowerCaps{Battery: p.r.HasBattery, Lid: p.r.Lid != "", Thermal: true}
}

func (p *Power) Read() (awake.Reading, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.none {
		return awake.Reading{}, contract.ErrUnsupported
	}
	return p.r, nil
}

func (p *Power) Set(r awake.Reading) {
	// invariant: Set moves the reading and tells every watch which fields moved.
	p.mu.Lock()
	was := p.r
	p.r = r
	watchers := append([]chan awake.Event(nil), p.watchers...)
	p.mu.Unlock()
	var kinds []string
	if was.Thermal != r.Thermal {
		kinds = append(kinds, "thermal")
	}
	if was.Source != r.Source {
		kinds = append(kinds, "source")
	}
	if was.Lid != r.Lid {
		kinds = append(kinds, "lid")
	}
	if was.HasBattery != r.HasBattery || was.Percent != r.Percent {
		kinds = append(kinds, "battery")
	}
	for _, w := range watchers {
		for _, k := range kinds {
			select {
			case w <- awake.Event{Kind: k}:
			default:
			}
		}
	}
}

func (p *Power) Watch(ctx context.Context) <-chan awake.Event {
	out := make(chan awake.Event, 8)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.none {
		close(out)
		return out
	}
	p.watchers = append(p.watchers, out)
	context.AfterFunc(ctx, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		for i, w := range p.watchers {
			if w == out {
				p.watchers = append(p.watchers[:i], p.watchers[i+1:]...)
				close(out)
				return
			}
		}
	})
	return out
}

func (p *Power) Hold(string) (func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.none {
		return nil, contract.ErrUnsupported
	}
	return func() {}, nil
}

func (p *Power) Clamshell(bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.none {
		return contract.ErrUnsupported
	}
	return nil
}

func (p *Power) DisplaySleepNow(context.Context) error { return nil }
