package menubar

import (
	"slices"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/page"
	pageawake "github.com/jterrazz/studio-station/src/pages/awake"
)

// invariant: AW-21, E1: This Mac's Awake offers Lid closed and On battery
// only where its capabilities have them: a desktop Mac keeps its terms and
// no switch, a laptop has both.
func TestAwakeOffersTheSwitchesThisMacHas(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC)
	offered := func(caps machine.Capabilities) []string {
		r := awake.Report{Now: now, Daemon: awake.EngineRunning, Mac: "this", Here: true, Reading: awake.Reading{Source: "ac"}, Caps: caps}
		p := Awake(pageawake.Fleet{Now: now, Zone: time.UTC, Reports: []awake.Report{r}}, page.Context{Machine: "this"})
		ids := make([]string, 0, len(p.Actions))
		for _, a := range p.Actions {
			ids = append(ids, a.ID)
		}
		return ids
	}
	if got, want := offered(machine.Capabilities{Engine: true}), []string{"keep:1h", "keep:4h", "keep-until"}; !slices.Equal(got, want) {
		t.Fatalf("a desktop Mac offers %q, want %q", got, want)
	}
	if got, want := offered(machine.Capabilities{Engine: true, Lid: true, Battery: true}), []string{"keep:1h", "keep:4h", "keep-until", "lid", "battery"}; !slices.Equal(got, want) {
		t.Fatalf("a laptop offers %q, want %q", got, want)
	}
}
