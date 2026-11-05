package menubar

import (
	"maps"
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/page"
)

// invariant: EN-10: Stop everything names This Mac, whatever its name, and
// says a server running there is skipped and keeps the engine running; a
// server on another Mac is not its business.
func TestStopEverythingOnThisMacSaysItsServerIsSkipped(t *testing.T) {
	fleet := &machines.Fleet{Hosts: []machines.HostView{
		{Name: "jterrazz-macbook-pro", Here: true, VMs: []machines.VM{
			{Name: "infra", Lifetime: machines.AlwaysOn, State: machines.Running},
			{Name: "linux-1", Lifetime: machines.Throwaway, State: machines.Running},
		}},
		{Name: "jterrazz-mac-mini", VMs: []machines.VM{{Name: "db", Lifetime: machines.AlwaysOn, State: machines.Running}}},
	}}
	confirm := stopAll(Reading{Fleet: fleet}).Confirm
	if confirm.Question != "Stop everything on This Mac?" {
		t.Fatalf("question %q", confirm.Question)
	}
	said := make([]string, 0, len(confirm.Lines))
	for _, l := range confirm.Lines {
		said = append(said, l.Text)
	}
	want := []string{"linux-1 will stop; its disk is kept", "infra skipped (server)", "Awake ends, and the engine keeps running for infra"}
	if !slices.Equal(said, want) {
		t.Fatalf("lines %q, want %q", said, want)
	}
}

// invariant: PO-45: on Home, a workspace's row opens its page, the engine's
// watch of menubar.vm:<workspace>, on This Mac or another; a server's opens
// none.
func TestAWorkspacesRowOnHomeOpensItsPage(t *testing.T) {
	two := &machines.Fleet{Hosts: []machines.HostView{
		{Name: "jterrazz-macbook-pro", Here: true, VMs: []machines.VM{
			{Name: "infra", Lifetime: machines.AlwaysOn, State: machines.Running},
			{Name: "linux-1", Lifetime: machines.Throwaway, State: machines.Running},
		}},
		{Name: "jterrazz-mac-mini", VMs: []machines.VM{{Name: "api", Lifetime: machines.Permanent, State: machines.Stopped}}},
	}}
	got := map[string]string{}
	var walk func([]page.Item)
	walk = func(items []page.Item) {
		for _, it := range items {
			switch {
			case it.Row != nil:
				got[it.Row.ID] = it.Row.Opens
			case it.Group != nil:
				got[it.Group.Head.ID] = it.Group.Head.Opens
				walk(it.Group.Items)
			}
		}
	}
	walk(fleet(Reading{Fleet: two}).Items)
	want := map[string]string{"jterrazz-macbook-pro": "", "infra": "", "linux-1": "menubar.vm:linux-1", "jterrazz-mac-mini": "", "api": "menubar.vm:api"}
	if !maps.Equal(got, want) {
		t.Fatalf("rows open %q, want %q", got, want)
	}
}
