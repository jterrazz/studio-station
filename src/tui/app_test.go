package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func fleetOf(vms ...string) machines.Fleet {
	h := machines.HostView{Name: "macbook", Here: true, Reachable: true}
	for _, n := range vms {
		h.VMs = append(h.VMs, machines.VM{Name: n, Kind: catalog.Linux, Host: "macbook", Lifetime: machines.Lasting, State: machines.Running})
	}
	return machines.Fleet{Hosts: []machines.HostView{h}}
}

func view(f machines.Fleet) tab.View[pagefleet.Reading] {
	here := page.Context{Kind: "host"}
	v := tab.NewView("fleet", "Fleet", "hosts", here, nil, func(r pagefleet.Reading) page.Page { return pagefleet.Tab(r, here) }, nil)
	v.Browser = v.Browser.Show(pagefleet.Tab(pagefleet.Reading{Machines: &f}, here))
	return v
}

func TestTheCursorFollowsTheRowIDAcrossARefill(t *testing.T) {
	v := view(fleetOf("api"))
	for range 2 {
		v, _, _ = v.Update(key("down"))
	}
	if v.Browser.Cursor != "fleet/api" {
		t.Fatalf("cursor = %q, want fleet/api", v.Browser.Cursor)
	}
	refilled := fleetOf("aaa", "api")
	v.Browser = v.Browser.Show(pagefleet.Tab(pagefleet.Reading{Machines: &refilled}, page.Context{Kind: "host"}))
	if v.Browser.Cursor != "fleet/api" {
		t.Errorf("after a refill cursor = %q, want fleet/api", v.Browser.Cursor)
	}
	section, row, ok := v.Browser.Current()
	if !ok || row.ID != "api" || section.Items[0].Group == nil || section.Items[0].Group.Head.ID != "macbook" {
		t.Errorf("the host is not a group node heading its VMs: %+v", section.Items)
	}
}

func TestABusyTabReportsNoAction(t *testing.T) {
	v := view(fleetOf("api"))
	for range 2 {
		v, _, _ = v.Update(key("down"))
	}
	_, _, ev := v.Update(key("s"))
	if ev == nil || ev.Row != "api" {
		t.Fatalf("s on api reported %+v, want the start/stop action", ev)
	}
	busy, _ := v.Act(nil, func() tea.Msg { return nil })
	if _, _, ev := busy.Update(key("s")); ev != nil {
		t.Errorf("a busy tab reported %+v", ev)
	}
}

func TestAHostRestartAsksItsName(t *testing.T) {
	v := view(fleetOf())
	v, _ = v.Ask(pagefleet.RestartConfirm("macbook", true), func() tea.Msg { return nil })
	if v.Browser.Focus() == 0 || !v.Modal() {
		t.Fatal("the restart opened no modal")
	}
	_, cmd, _ := v.Update(key("y"))
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, started := msg.(tab.Done); started {
				t.Error("y confirmed a restart that asks the name")
			}
		}
	}
}
