package machines

import (
	"context"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	poweroffIntent = "station-intent=poweroff"
	rerunWindow    = 60 * time.Second
	rerunBudget    = 3
)

type NotTheAgentError struct{ Name string }

func (e *NotTheAgentError) Error() string {
	return "only the launchd agent of " + e.Name + " supervises it"
}

type GuestStopsError struct{ Name string }

func (e *GuestStopsError) Error() string {
	return e.Name + " stopped " + strconv.Itoa(rerunBudget+1) + " times within " + rerunWindow.String() + " without a poweroff"
}

type RunEndedError struct {
	Name string
	Code int
}

func (e *RunEndedError) Error() string {
	return "the run of " + e.Name + " ended with exit " + strconv.Itoa(e.Code) + ": nothing was deleted"
}

type action int

const (
	actRerun action = iota
	actDelete
	actDone
	actGuestStops
	actCrashed
)

// invariant: a temporary VM is deleted only on a poweroff its guest declared; a
// reboot (a clean stop without the intent) runs it again, and every other stop
// keeps it, stale, for `clean stale-vms` or `delete` to end on purpose.
func next(l Lifetime, intent bool, code int, reruns []time.Time, now time.Time) (action, []time.Time) {
	switch {
	case intent && l == Temporary:
		return actDelete, reruns
	case intent:
		return actDone, reruns
	case code != 0:
		return actCrashed, reruns
	}
	recent := reruns[:0:0]
	for _, at := range reruns {
		if now.Sub(at) < rerunWindow {
			recent = append(recent, at)
		}
	}
	if len(recent) >= rerunBudget {
		return actGuestStops, recent
	}
	return actRerun, append(recent, now)
}

func (s *Service) Supervise(ctx context.Context, name string) error {
	// invariant: the VM's launchd agent runs this loop: the VM runs attached, then
	// its console says whether the guest declared a poweroff.
	if err := s.CheckSupervise(name); err != nil {
		return err
	}
	rec, err := s.VM(name, "supervise")
	if err != nil {
		return err
	}
	console := s.consolePath(name)
	if err := s.h.FS.MkdirAll(path.Dir(console), 0o700); err != nil {
		return err
	}
	var reruns []time.Time
	for {
		if err := s.h.FS.WriteFile(console, nil, 0o600); err != nil {
			return err
		}
		argv := s.engine.RunArgv(name, console)
		ran, err := s.h.Run.Attach(ctx, system.Cmd{Name: argv[0], Args: argv[1:]}, system.ToStderr)
		code := ran.ExitCode
		if err != nil {
			return err
		}
		var act action
		act, reruns = next(rec.Lifetime, s.declaredPoweroff(console), code, reruns, s.h.Clock.Now())
		switch act {
		case actRerun:
			continue
		case actDelete:
			_, err := s.Delete(ctx, name)
			return err
		case actDone:
			return nil
		case actGuestStops:
			return &GuestStopsError{Name: name}
		case actCrashed:
			return &RunEndedError{Name: name, Code: code}
		}
	}
}

func (s *Service) CheckSupervise(name string) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	if s.o.Agent != name {
		return &NotTheAgentError{Name: name}
	}
	return nil
}

func (s *Service) declaredPoweroff(console string) bool {
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
