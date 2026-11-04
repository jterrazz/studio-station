package local

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/platform/darwin"
)

const watchEvery = time.Second

var ErrPowerUnsupported = errors.New("no power management here: macOS only")

type PowerReading = darwin.Reading

type Power interface {
	// value: the door to this Mac's power management an adapter is given: an
	// idle assertion, the clamshell flag, a reading, and how a watch learns that
	// the reading moved.
	HoldIdle(name string) (release func(), err error)
	SetClamshellSleep(disabled bool) error
	Read() (PowerReading, error)
	// invariant: one tick at every period and at every post that the
	// reading may have moved, until ctx ends; nil where there is no power
	// management.
	Ticks(ctx context.Context) <-chan struct{}
}

func ticks(ctx context.Context, every time.Duration, wake <-chan struct{}) <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-wake:
			}
			select {
			case out <- struct{}{}:
			default:
			}
		}
	}()
	return out
}

func NewPower(env []string) Power {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	if dir := vars["STATION_POWER_STUB"]; filepath.IsAbs(dir) {
		return &stubPower{dir: dir, calls: filepath.Join(identity{vars: vars}.Home(), ".calls")}
	}
	if !darwin.Supported {
		return noPower{}
	}
	return &macPower{}
}

type macPower struct {
	iokit darwin.Power
}

func (p *macPower) HoldIdle(name string) (func(), error) { return darwin.HoldIdle(name) }

func (p *macPower) SetClamshellSleep(disabled bool) error {
	return darwin.SetClamshellSleep(disabled)
}

func (p *macPower) Read() (PowerReading, error) { return p.iokit.Read(), nil }

func (p *macPower) Ticks(ctx context.Context) <-chan struct{} {
	return ticks(ctx, watchEvery, darwin.PowerSourcesChanged(ctx))
}

type noPower struct{}

func (noPower) HoldIdle(string) (func(), error) { return nil, ErrPowerUnsupported }

func (noPower) SetClamshellSleep(bool) error { return ErrPowerUnsupported }

func (noPower) Read() (PowerReading, error) { return PowerReading{}, ErrPowerUnsupported }

func (noPower) Ticks(context.Context) <-chan struct{} { return nil }
