package engine

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestTheEventsWatchStreamsWhatTheEngineTellsOnceEach(t *testing.T) {
	d := &Daemon{o: Options{Join: func(parts ...string) string { return strings.Join(parts, " / ") }}}
	ctx, cancel := context.WithCancel(t.Context())
	got := make(chan Event, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Events(ctx, func(v any) error {
			ev, _ := v.(Event)
			got <- ev
			return nil
		})
	}()
	for !d.hub.listening() {
		runtime.Gosched()
	}
	d.tell(ctx, "infra", Back("infra"))
	d.tell(ctx, "infra", PoweredOff("infra"))
	for _, want := range []Notice{{Kind: EventBack, Text: "infra is back / all 4 health steps pass"}, {Kind: EventPoweredOff, Text: "infra powered off from inside / it stays stopped"}} {
		if ev := <-got; ev.Event != "notice" || *ev.Notice != want {
			t.Errorf("streamed %+v, want the notice %+v", ev, want)
		}
	}
	cancel()
	<-done
	select {
	case ev := <-got:
		t.Errorf("streamed %+v once more", ev)
	default:
	}
}
