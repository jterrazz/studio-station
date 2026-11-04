package awake

import "time"

const DefaultFor = 4 * time.Hour

type Request struct {
	Lid     bool
	Battery bool
	For     time.Duration
	By      string
}

type Session struct {
	Lid       bool      `json:"lid"`
	Battery   bool      `json:"battery"`
	StartedAt time.Time `json:"started_at"`
	Until     time.Time `json:"until"`
	By        string    `json:"by"`
}

type End struct {
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
	Detail string    `json:"detail"`
}

type GuardState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type View struct {
	Session *Session     `json:"session"`
	LastEnd *End         `json:"last_end"`
	Reading Reading      `json:"reading"`
	Guards  []GuardState `json:"guards"`
}

type Slept struct {
	Ended bool          `json:"ended"`
	After time.Duration `json:"after"`
}

type Report struct {
	Now time.Time
	// value: the zone a page shows its clock times in (the owner's, read once
	// by the caller); nil is UTC.
	Zone    *time.Location
	Daemon  string
	Reading Reading
	Session *Session
	LastEnd *End
	Guards  []GuardState
}

type Awoken struct {
	Session
	Refreshed bool `json:"refreshed"`
}
