package activity

import (
	"time"

	"github.com/jterrazz/studio-station/src/core/finding"
)

type Source string

const (
	// value: the history of the engine that keeps the machine's activity.
	FromEngine Source = "engine"
	// value: two samples 1 s apart, read now: the engine keeps none.
	FromDirect Source = "direct"
	// value: a VM read from its Mac while its guest does not answer: its
	// process there, nothing of its own counters.
	FromMacProcess Source = "mac-process"
	// value: the engine of this Mac keeps no history of the machine asked.
	NotKept Source = ""
)

const (
	// value: the window a rate is the mean of when the engine holds
	// history (SPEC §6.2); a direct read's two samples are DirectGap apart.
	RateWindow = 10 * time.Second
	DirectGap  = time.Second
	// value: what a read waits for a Meter before it says it did not answer.
	Budget = 2 * time.Second
)

type Report struct {
	Source Source `json:"source"`
	// value: when the engine that keeps the history started; zero on a
	// direct read.
	Since time.Time `json:"since,omitzero"`
	Last  Reading   `json:"last"`
	// value: the means over Window; Window zero when no two points span one.
	Rates  Rates         `json:"rates"`
	Window time.Duration `json:"window"`
	// value: the Meter did not answer in Budget: no value is known.
	Silent bool `json:"silent,omitempty"`
	// value: the coarse series, when the read asked for the history.
	History []Sample `json:"history,omitempty"`
	// invariant: never nil: SPEC §6.5's findings over the series read.
	Findings []finding.Finding `json:"findings"`
}

func SeriesOf(fine, coarse []Sample) Series {
	// value: the series a fine and a coarse window hold, oldest first, as
	// the engine hands them over its API.
	var s Series
	for _, x := range fine {
		s.fine.add(x, FinePoints)
	}
	for _, x := range coarse {
		s.coarse.add(x, CoarsePoints)
	}
	return s
}

func Recent(s *Series) (Rates, time.Duration) {
	// invariant: the mean from the newest point at least RateWindow before
	// the newest one, or from the oldest when none is that old; a series of
	// one point has no rate.
	points := merged(s)
	if len(points) < 2 {
		return Rates{}, 0
	}
	newest := points[len(points)-1]
	base := points[0]
	for i := len(points) - 2; i >= 0; i-- {
		if newest.At.Sub(points[i].At) >= RateWindow {
			base = points[i]
			break
		}
	}
	return Between(base, newest), newest.At.Sub(base.At)
}
