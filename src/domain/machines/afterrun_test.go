package machines

import (
	"testing"
	"time"
)

func TestAfterRunKeepsEveryVMAStopDidNotDeclare(t *testing.T) {
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	for _, c := range []struct {
		name     string
		lifetime Lifetime
		intent   bool
		code     int
		reruns   []time.Time
		now      time.Time
		want     Action
	}{
		{name: "the intent wins over the run's exit code", lifetime: Temporary, intent: true, code: 1, now: at(0), want: ActDelete},
		{name: "a lasting VM's declared poweroff keeps it stopped", lifetime: Lasting, intent: true, now: at(0), want: ActStopped},
		{name: "a temporary crash keeps it stale", lifetime: Temporary, code: 137, now: at(0), want: ActStale},
		{name: "a lasting crash runs again later", lifetime: Lasting, code: 137, now: at(0), want: ActRerunLater},
		{name: "a third re-run within the window still runs", lifetime: Temporary, reruns: []time.Time{at(0), at(10)}, now: at(20), want: ActRerun},
		{name: "a fourth stop within the window is broken", lifetime: Temporary, reruns: []time.Time{at(0), at(10), at(20)}, now: at(30), want: ActBroken},
		{name: "a lasting crash loop is broken too", lifetime: Lasting, code: 1, reruns: []time.Time{at(0), at(10), at(20)}, now: at(30), want: ActBroken},
		{name: "re-runs older than the window are forgotten", lifetime: Temporary, reruns: []time.Time{at(0), at(10), at(20)}, now: at(65), want: ActRerun},
		{name: "the window is a strict 60 seconds", lifetime: Temporary, reruns: []time.Time{at(0), at(30), at(40)}, now: at(60), want: ActRerun},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := AfterRun(c.lifetime, c.intent, c.code, c.reruns, c.now); got != c.want {
				t.Errorf("AfterRun = %d, want %d", got, c.want)
			}
		})
	}
}
