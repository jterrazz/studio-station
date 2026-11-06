package machines_test

import (
	"context"
	"slices"
	"testing"
	"time"

	corefake "github.com/jterrazz/studio-station/src/core/adapter/fake"
	model "github.com/jterrazz/studio-station/src/machines"
	machinesfake "github.com/jterrazz/studio-station/src/machines/adapter/fake"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func fleetAsking(t *testing.T, ask func(ctx context.Context, macs []string, answered func(string, machines.HostView))) *machines.Service {
	t.Helper()
	h := system.Host{Run: systemtest.NewRunner(t), FS: systemtest.NewFS(), Env: systemtest.Env{HomeDir: "/home"}, Net: systemtest.NewNet(), Clock: &systemtest.Clock{At: time.Unix(0, 0)}}
	prefs := corefake.NewPreferences()
	p := machines.Prefs{Name: setupcontract.Name(prefs), Hosts: machinescontract.Macs(prefs), VMUser: machinescontract.VMUser(prefs)}
	if err := p.Name.Set("laptop"); err != nil {
		t.Fatal(err)
	}
	if err := p.Hosts.Set(map[string]model.MacRecord{"laptop": {}, "mac-mini": {}, "studio": {}}); err != nil {
		t.Fatal(err)
	}
	return machines.New(h, nil, p, machinesfake.NewStore(), noTailnet{}, nil, nil, nil, machines.Options{Here: catalog.Mac, Ask: ask})
}

func names(f machines.Fleet) []string {
	out := make([]string, 0, len(f.Macs))
	for _, h := range f.Macs {
		out = append(out, h.Name)
	}
	return out
}

func TestTheFleetFillsARowAsEachMacAnswersAndASilentOneEndsUnreachable(t *testing.T) {
	svc := fleetAsking(t, func(_ context.Context, _ []string, answered func(string, machines.HostView)) {
		answered("studio", machines.HostView{Name: "studio"})
	})
	var seen [][]string
	fleet, err := svc.FleetAsItFills(t.Context(), false, func(f machines.Fleet) { seen = append(seen, names(f)) })
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"laptop"}, {"laptop", "studio"}}
	if !slices.EqualFunc(seen, want, slices.Equal[[]string]) {
		t.Errorf("the rows as they filled: %q, want %q: this Mac at once, the studio when it answered, the silent mini not yet", seen, want)
	}
	got := names(fleet)
	if !slices.Equal(got, []string{"laptop", "mac-mini", "studio"}) || fleet.Macs[1].Reachable || !fleet.Macs[2].Reachable {
		t.Errorf("the fleet at the end: %q, mini reachable %v; want every Mac, in name order, the silent mini unreachable", got, fleet.Macs[1].Reachable)
	}
}
