package host

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

func watchReadings(ctx context.Context, read func() (system.PowerReading, error), every time.Duration) <-chan system.PowerEvent {
	out := make(chan system.PowerEvent, 8)
	go func() {
		defer close(out)
		last, _ := read()
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			now, err := read()
			if err != nil {
				continue
			}
			for _, kind := range moved(last, now) {
				select {
				case out <- system.PowerEvent{Kind: kind}:
				case <-ctx.Done():
					return
				}
			}
			last = now
		}
	}()
	return out
}

func moved(a, b system.PowerReading) []string {
	var kinds []string
	if a.Thermal != b.Thermal {
		kinds = append(kinds, "thermal")
	}
	if a.Source != b.Source {
		kinds = append(kinds, "source")
	}
	if a.Lid != b.Lid {
		kinds = append(kinds, "lid")
	}
	return kinds
}
