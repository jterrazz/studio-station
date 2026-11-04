package machines

import (
	"testing"
	"time"
)

func TestAfterRunKeepsEveryVMAStopDidNotDeclare(t *testing.T) {
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	poweroff := RunEnd{Reason: GuestPoweroff}
	reboot := RunEnd{Reason: GuestReboot}
	for _, c := range []struct {
		name     string
		lifetime Lifetime
		end      RunEnd
		reruns   []time.Time
		now      time.Time
		want     Action
	}{
		{name: "the declared poweroff wins over the run's exit code", lifetime: Temporary, end: RunEnd{Reason: GuestPoweroff, ExitCode: 1}, now: at(0), want: ActDelete},
		{name: "a lasting VM's declared poweroff keeps it stopped", lifetime: Lasting, end: poweroff, now: at(0), want: ActStopped},
		{name: "a temporary crash keeps it stale", lifetime: Temporary, end: RunEnd{Reason: Crashed, ExitCode: 137}, now: at(0), want: ActStale},
		{name: "a temporary run whose end nobody can read keeps it stale", lifetime: Temporary, end: RunEnd{Reason: Unknown, ExitCode: -1}, now: at(0), want: ActStale},
		{name: "a lasting crash runs again later", lifetime: Lasting, end: RunEnd{Reason: Crashed, ExitCode: 137}, now: at(0), want: ActRerunLater},
		{name: "a third re-run within the window still runs", lifetime: Temporary, end: reboot, reruns: []time.Time{at(0), at(10)}, now: at(20), want: ActRerun},
		{name: "a fourth stop within the window is broken", lifetime: Temporary, end: reboot, reruns: []time.Time{at(0), at(10), at(20)}, now: at(30), want: ActBroken},
		{name: "a lasting crash loop is broken too", lifetime: Lasting, end: RunEnd{Reason: Crashed, ExitCode: 1}, reruns: []time.Time{at(0), at(10), at(20)}, now: at(30), want: ActBroken},
		{name: "re-runs older than the window are forgotten", lifetime: Temporary, end: reboot, reruns: []time.Time{at(0), at(10), at(20)}, now: at(65), want: ActRerun},
		{name: "the window is a strict 60 seconds", lifetime: Temporary, end: reboot, reruns: []time.Time{at(0), at(30), at(40)}, now: at(60), want: ActRerun},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := AfterRun(c.lifetime, c.end, c.reruns, c.now); got != c.want {
				t.Errorf("AfterRun = %d, want %d", got, c.want)
			}
		})
	}
}
