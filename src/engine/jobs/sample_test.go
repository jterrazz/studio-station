package jobs

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type machines struct {
	clock *clock
	mu    sync.Mutex
	asked map[string]int
}

func (m *machines) Invoke(_ context.Context, target machine.Ref, _ string, _, out any, _ call.Progress) error {
	m.mu.Lock()
	m.asked[target.Name]++
	m.mu.Unlock()
	r, ok := out.(*activity.Reading)
	if !ok {
		return errors.New("activity.sample answers a Reading")
	}
	*r = activity.Reading{Sample: activity.Sample{At: m.clock.Now()}}
	return nil
}

func (m *machines) Watch(context.Context, machine.Ref, string, any, call.Progress) (<-chan call.Update[any], error) {
	return nil, nil
}

func TestTwentyIdleVMsAreSampledOnceAMinuteEach(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)}
	fleet := &machines{clock: c, asked: map[string]int{}}
	seen := make([]Seen, 0, 21)
	seen = append(seen, Seen{Ref: machine.Ref{Name: "mac"}, Kind: machine.Mac})
	for i := range 20 {
		seen = append(seen, Seen{Ref: machine.Ref{Name: "vm-" + strconv.Itoa(i), On: "mac"}, Kind: machine.Workspace, PID: 1000 + i, Guest: true})
	}
	watched := false
	h := activity.NewHistory(c.now)
	s := &Sample{
		Calls: fleet, Clock: c, Seen: func() []Seen { return seen }, Watched: func() bool { return watched },
		History: func() *activity.History { return h }, Tell: func(string, string) {},
	}
	tick := func(seconds int) {
		for range seconds {
			s.Run(t.Context())
			c.now = c.now.Add(time.Second)
		}
	}
	tick(600)
	for _, m := range seen {
		if got := fleet.asked[m.Ref.Name]; got != 10 {
			t.Errorf("%s was sampled %d times in 10 idle minutes; want 10, once a minute", m.Ref.Name, got)
		}
	}
	watched = true
	tick(60)
	for _, m := range seen {
		if got := fleet.asked[m.Ref.Name] - 10; got != 30 {
			t.Errorf("%s was sampled %d times in a watched minute; want 30, every 2 s", m.Ref.Name, got)
		}
	}
}
