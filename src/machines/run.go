package machines

import (
	"strconv"
	"time"
)

type Lifetime string

const (
	Temporary Lifetime = "temporary"
	Lasting   Lifetime = "lasting"
)

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
	rerunBudget = 3
	CrashDelay  = 10 * time.Second
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

func AfterRun(l Lifetime, end RunEnd, reruns []time.Time, now time.Time) (Action, []time.Time) {
	// invariant: a temporary VM is deleted only on a poweroff its guest declared; a
	// reboot runs it again, at most 3 times in 60 s; a crash or an end nobody can
	// read keeps a temporary VM, stale, and runs a lasting one again after 10 s.
	clean := end.Reason == GuestReboot
	switch {
	case end.Reason == GuestPoweroff && l == Temporary:
		return ActDelete, reruns
	case end.Reason == GuestPoweroff:
		return ActStopped, reruns
	case !clean && l == Temporary:
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
	if !clean {
		return ActRerunLater, append(recent, now)
	}
	return ActRerun, append(recent, now)
}
