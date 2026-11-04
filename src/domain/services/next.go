package services

import (
	"github.com/jterrazz/studio-station/src/ports"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

type Phase = ports.Phase

const (
	None      = ports.PhaseNone
	Published = ports.PhasePublished
	Closing   = ports.PhaseClosing
	Waiting   = ports.PhaseWaiting
	Refused   = ports.PhaseRefused
	Gone      = ports.PhaseGone
	Unknown   = ports.PhaseUnknown
)

type Event int

const (
	EvListen Event = iota
	EvUnlisten
	EvTick
	EvVMStopped
	EvDeleted
	EvReleased
)

type Action int

const (
	ActPublish Action = iota
	ActUnpublish
	ActRelease
)

type State struct {
	Phase      Phase
	Since      time.Time
	Pinned     bool
	LastSeenAt time.Time
}

func Next(st State, ev Event, now time.Time) (State, []Action) {
	// invariant: the lifecycle of one service (N5): a stop of listening unpublishes
	// only after the grace, a re-listen inside it keeps everything; a lease outlives
	// its listener and idles out after 7 days unless pinned.
	switch ev {
	case EvListen:
		st.LastSeenAt = now
		switch st.Phase {
		case Closing:
			st.Phase = Published
			return st, nil
		case Published:
			return st, nil
		case None, Waiting, Refused, Gone, Unknown:
		}
		st.Phase, st.Since = Published, now
		return st, []Action{ActPublish}
	case EvUnlisten:
		if st.Phase == Published {
			st.Phase, st.Since = Closing, now
		}
		return st, nil
	case EvTick:
		switch {
		case st.Phase == Closing && now.Sub(st.Since) >= catalog.ServicePolicy.Grace:
			st.Phase, st.Since = Waiting, now
			return st, []Action{ActUnpublish}
		case st.Phase == Waiting && !st.Pinned && !st.LastSeenAt.IsZero() && now.Sub(st.LastSeenAt) > catalog.ServicePolicy.Idle:
			st.Phase = Gone
			return st, []Action{ActRelease}
		}
		return st, nil
	case EvVMStopped:
		if st.Phase == Published || st.Phase == Closing {
			st.Phase, st.Since = Waiting, now
			return st, []Action{ActUnpublish}
		}
		if st.Phase == None {
			st.Phase = Waiting
		}
		return st, nil
	case EvDeleted, EvReleased:
		was := st.Phase
		st.Phase = Gone
		if was == Published || was == Closing {
			return st, []Action{ActUnpublish, ActRelease}
		}
		return st, []Action{ActRelease}
	}
	return st, nil
}
