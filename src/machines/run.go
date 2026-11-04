package machines

import "time"

type Lifetime string

const (
	Throwaway Lifetime = "throwaway"
	Permanent Lifetime = "permanent"
)

func (l *Lifetime) UnmarshalText(text []byte) error {
	// invariant: a record station v2 wrote reads in v3's words: temporary
	// is throwaway, lasting is permanent; the next write keeps the new word.
	switch v := Lifetime(text); v {
	case "temporary":
		*l = Throwaway
	case "lasting":
		*l = Permanent
	default:
		*l = v
	}
	return nil
}

type Size struct {
	CPUs         int
	Memory, Disk int64
}

type Network struct {
	// value: on its Mac's network, through the Mac's default interface;
	// false keeps it behind its Mac.
	Bridged bool
}

type EndReason string

const (
	GuestPoweroff EndReason = "poweroff"
	GuestReboot   EndReason = "reboot"
	Crashed       EndReason = "crashed"
	Unknown       EndReason = "unknown"
)

type RunEnd struct {
	Reason   EndReason
	ExitCode int
	Detail   string
}

const (
	rerunWindow = 60 * time.Second
	rerunBudget = 2
)

type Action int

const (
	ActRerun Action = iota
	ActDelete
	ActStopped
	ActBroken
	ActStale
)

func BrokenDetail() string { return "rebooting in a loop · stopped" }

func AfterRun(l Lifetime, end RunEnd, reruns []time.Time, now time.Time) (Action, []time.Time) {
	// invariant: a workspace never restarts by itself (SP-4). A poweroff its
	// guest declared deletes a throwaway one and stops a permanent one; a
	// reboot from inside runs it again, and the third boot within 60 s that
	// ends in a reboot is a loop, stopped (SPEC §7.3); a crash or
	// an end nobody can read keeps it, stale, for the owner to start or delete.
	switch end.Reason {
	case GuestPoweroff:
		if l == Throwaway {
			return ActDelete, reruns
		}
		return ActStopped, reruns
	case GuestReboot:
	default:
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
	return ActRerun, append(recent, now)
}
