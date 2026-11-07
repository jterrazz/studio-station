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
		r := awake.Report{Now: now, Engine: awake.EngineRunning, Mac: "this", ThisMac: true, Reading: awake.Reading{Source: "ac"}, Caps: caps}
		p := Awake(pageawake.AllMachines{Now: now, Zone: time.UTC, Reports: []awake.Report{r}}, page.Context{Machine: "this"})
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

// invariant: Keep awake for… shows the term in force: the one the session
// was taken for, else Until; nothing while This Mac sleeps normally.
func TestKeepAwakeForPicksTheTermInForce(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC)
	picked := func(s *awake.Session) []string {
		r := awake.Report{Now: now, Engine: awake.EngineRunning, Mac: "this", ThisMac: true, Reading: awake.Reading{Source: "ac"}, Caps: machine.Capabilities{Engine: true}, Session: s}
		p := Awake(pageawake.AllMachines{Now: now, Zone: time.UTC, Reports: []awake.Report{r}}, page.Context{Machine: "this"})
		var ids []string
		for _, a := range p.Actions {
			if a.Picked {
				ids = append(ids, a.ID)
			}
		}
		return ids
	}
	if got := picked(&awake.Session{StartedAt: now.Add(-3 * time.Minute), Until: now.Add(57 * time.Minute)}); !slices.Equal(got, []string{"keep:1h"}) {
		t.Fatalf("a 1 h session picks %q, want keep:1h", got)
	}
	if got := picked(&awake.Session{StartedAt: now.Add(-20 * time.Minute), Until: now.Add(4 * time.Hour), For: 4 * time.Hour}); !slices.Equal(got, []string{"keep:4h"}) {
		t.Fatalf("a 4 h term asked again picks %q, want keep:4h", got)
	}
	if got := picked(&awake.Session{StartedAt: now, Until: now.Add(150 * time.Minute)}); !slices.Equal(got, []string{"keep-until"}) {
		t.Fatalf("a session until 13:00 picks %q, want keep-until", got)
	}
	if got := picked(nil); len(got) != 0 {
		t.Fatalf("no session picks %q, want none", got)
	}
}

// invariant: Until's hours are sent as the hour itself, and a switch
// flipped while a keep-awake holds sends the end it holds until, never a
// time left counted when the page was read: both end where they said.
func TestAwakeSendsTheEndItHoldsUntil(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC)
	until := time.Date(2026, 11, 3, 14, 30, 0, 0, time.UTC)
	r := awake.Report{Now: now, Engine: awake.EngineRunning, Mac: "this", ThisMac: true, Reading: awake.Reading{Source: "ac"},
		Caps:    machine.Capabilities{Engine: true, Lid: true, Battery: true},
		Session: &awake.Session{StartedAt: now.Add(-time.Hour), Until: until, For: 4 * time.Hour}}
	p := Awake(pageawake.AllMachines{Now: now, Zone: time.UTC, Reports: []awake.Report{r}}, page.Context{Machine: "this"})
	for _, a := range p.Actions {
		switch a.ID {
		case ActionUntil:
			f := a.Form.Fields[0]
			if f.Key != "Until" || f.Options[0].Value != `"2026-11-03T11:00:00Z"` || f.Options[0].Label != "11:00" {
				t.Fatalf("Until offers %s = %s (%s)", f.Key, f.Options[0].Value, f.Options[0].Label)
			}
		case ActionLid, ActionBattery:
			if a.Run.In["Until"] != `"2026-11-03T14:30:00Z"` || a.Run.In["For"] != "" {
				t.Fatalf("%s sends %v", a.ID, a.Run.In)
			}
		}
	}
}

// invariant: under Keep awake, This Mac's line says the end without the
// title's word (vision panel e: `until 18:00 · lid closed · 3 h 58 min left`).
func TestKeepAwakeSaysTheEndWithoutAwake(t *testing.T) {
	now := time.Date(2026, 11, 3, 14, 2, 0, 0, time.UTC)
	s := &awake.Session{StartedAt: now, Until: now.Add(3*time.Hour + 58*time.Minute), Lid: true}
	r := awake.Report{Now: now, Engine: awake.EngineRunning, Mac: "this", ThisMac: true, Reading: awake.Reading{Source: "ac"}, Caps: machine.Capabilities{Engine: true, Lid: true}, Session: s}
	p := Awake(pageawake.AllMachines{Now: now, Zone: time.UTC, Reports: []awake.Report{r}}, page.Context{Machine: "this"})
	row := p.Sections[0].Items[0].Row
	words := make([]string, 0, len(row.Cells[0].Items))
	for _, v := range row.Cells[0].Items {
		words = append(words, v.Text)
	}
	if want := []string{"until 18:00", "lid closed", "3h 58m left"}; !slices.Equal(words, want) {
		t.Fatalf("This Mac's line says %q, want %q", words, want)
	}
}
