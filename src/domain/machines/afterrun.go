package machines

import (
	"strconv"
	"strings"
	"time"
)

const (
	poweroffIntent = "station-intent=poweroff"
	rerunWindow    = 60 * time.Second
	rerunBudget    = 3
	CrashDelay     = 10 * time.Second
)

type Action int

const (
	ActRerun Action = iota
	ActRerunLater
	ActDelete
	ActStopped
	ActBroken
	ActStale
)

func BrokenDetail() string {
	return "stopped " + strconv.Itoa(rerunBudget+1) + " times within " + strconv.Itoa(int(rerunWindow.Seconds())) + " s"
}

func AfterRun(l Lifetime, intent bool, code int, reruns []time.Time, now time.Time) (Action, []time.Time) {
	// invariant: a temporary VM is deleted only on a poweroff its guest declared; a
	// reboot (a clean end without the intent) runs it again, at most 3 times in 60 s;
	// a crash keeps a temporary VM, stale, and runs a lasting one again after 10 s.
	switch {
	case intent && l == Temporary:
		return ActDelete, reruns
	case intent:
		return ActStopped, reruns
	case code != 0 && l == Temporary:
		return ActStale, reruns
	}
	recent := reruns[:0:0]
	for _, at := range reruns {
		if now.Sub(at) < rerunWindow {
			recent = append(recent, at)
		}
	}
	if len(recent) >= rerunBudget {
		return ActBroken, recent
	}
	if code != 0 {
		return ActRerunLater, append(recent, now)
	}
	return ActRerun, append(recent, now)
}

func (s *Service) DeclaredPoweroff(console string) bool {
	data, err := s.h.FS.ReadFile(console)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == poweroffIntent {
			return true
		}
	}
	return false
}
