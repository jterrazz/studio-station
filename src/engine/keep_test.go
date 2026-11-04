package engine

import (
	"context"
	"testing"
	"time"
)

func TestTheEventsWatchStreamsWhatTheEngineTellsOnceEach(t *testing.T) {
	d := &Daemon{o: Options{Join: func(parts ...string) string {
		out := parts[0]
		for _, p := range parts[1:] {
			out += " / " + p
		}
		return out
	}}}
	ctx, cancel := context.WithCancel(t.Context())
	got := make(chan Event, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Events(ctx, func(v any) error {
			got <- v.(Event)
			return nil
		})
	}()
	for !d.hub.listening() {
		time.Sleep(time.Millisecond)
	}
	kind, parts := Back("infra")
	d.tell(ctx, "infra", kind, parts)
	kind, parts = PoweredOff("infra")
	d.tell(ctx, "infra", kind, parts)
	for _, want := range []Notice{{Kind: EventBack, Text: "infra is back / all 4 health steps pass"}, {Kind: EventPoweredOff, Text: "infra powered off from inside / it stays stopped"}} {
		select {
		case ev := <-got:
			if ev.Event != "notice" || *ev.Notice != want {
				t.Errorf("streamed %+v, want the notice %+v", ev, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("the watch did not stream %q", want.Text)
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
