package machines

import (
	"testing"
	"time"
)

func TestNextKeepsEveryVMAStopDidNotDeclare(t *testing.T) {
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	for _, c := range []struct {
		name     string
		lifetime Lifetime
		intent   bool
		code     int
		reruns   []time.Time
		now      time.Time
		want     action
	}{
		{name: "the intent wins over the run's exit code", lifetime: Temporary, intent: true, code: 1, now: at(0), want: actDelete},
		{name: "a lasting crash is launchd's to restart", lifetime: Lasting, code: 137, now: at(0), want: actCrashed},
		{name: "a third re-run within the window still runs", lifetime: Temporary, reruns: []time.Time{at(0), at(10)}, now: at(20), want: actRerun},
		{name: "re-runs older than the window are forgotten", lifetime: Temporary, reruns: []time.Time{at(0), at(10), at(20)}, now: at(65), want: actRerun},
		{name: "the window is a strict 60 seconds", lifetime: Temporary, reruns: []time.Time{at(0), at(30), at(40)}, now: at(60), want: actRerun},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := next(c.lifetime, c.intent, c.code, c.reruns, c.now)
			if got != c.want {
				t.Errorf("next = %d, want %d", got, c.want)
			}
		})
	}
}

func TestNextCountsEachRerunOnce(t *testing.T) {
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	var reruns []time.Time
	var got action
	for i := range 4 {
		got, reruns = next(Temporary, false, 0, reruns, start.Add(time.Duration(i)*time.Second))
	}
	if got != actGuestStops {
		t.Errorf("the fourth clean stop in a row = %d, want actGuestStops", got)
	}
}
