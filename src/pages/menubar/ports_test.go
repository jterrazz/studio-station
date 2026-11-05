package menubar

import (
	"slices"
	"testing"

	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ports"
)

// invariant: MB-17: on the Ports of every Mac, Review on a red row of
// mac-mini opens a terminal on station ports --on mac-mini, and Review on
// one of This Mac's on station ports.
func TestReviewOnEveryMacsPortsNamesTheMacOfItsRow(t *testing.T) {
	red := func(mac string, port int) ports.Port {
		return ports.Port{Machine: coremachine.Ref{Name: mac}, Port: port, Process: "p", Binds: []string{"*"}, Reach: ports.Outside}
	}
	l := ports.Listing{Fleet: services.Fleet{Hosts: []services.HostView{
		{Name: "jterrazz-macbook-pro", Here: true, Reachable: true, Services: []services.View{}, Ports: []ports.Port{red("jterrazz-macbook-pro", 5000)}},
		{Name: "jterrazz-mac-mini", Reachable: true, Services: []services.View{}, Ports: []ports.Port{red("jterrazz-mac-mini", 10250)}},
	}}}
	p := Ports(l, PortsOf{}, page.Context{Machine: "jterrazz-macbook-pro"})
	var got [][]string
	for _, a := range p.Actions {
		if a.Run != nil && a.Run.Terminal {
			got = append(got, append(append([]string{a.ID}, a.Rows...), a.Run.Argv...))
		}
	}
	want := [][]string{
		{"review:jterrazz-macbook-pro", "jterrazz-macbook-pro:5000", "ports"},
		{"review:jterrazz-mac-mini", "jterrazz-mac-mini:10250", "ports", "--on", "jterrazz-mac-mini"},
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("reviews %q, want %q", got, want)
	}
}

// invariant: PO-45: the Ports page of a Mac offers Declare public and
// never Share a port, which is a workspace's page's own.
func TestTheMacsPortsPageNeverOffersShareAPort(t *testing.T) {
	l := ports.Listing{Fleet: services.Fleet{Hosts: []services.HostView{
		{Name: "jterrazz-macbook-pro", Here: true, Reachable: true, Services: []services.View{
			{VM: "linux-1", GuestPort: 5173, HostPort: 40001, State: services.Published, Process: "node", URLs: services.URLSet{Public: "https://mac.ts.net:40001"}},
		}},
	}}}
	mac := Ports(l, PortsOf{}, page.Context{Machine: "jterrazz-macbook-pro"})
	if action(mac, ActionShare) != nil || action(mac, ActionDeclarePublic) == nil {
		t.Fatalf("the Ports page of every Mac offers %v", mac.Actions)
	}
}

func action(p page.Page, id string) *page.Action {
	for i := range p.Actions {
		if p.Actions[i].ID == id {
			return &p.Actions[i]
		}
	}
	return nil
}
