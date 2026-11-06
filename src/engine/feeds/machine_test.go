package feeds_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	"github.com/jterrazz/studio-station/src/activity"
	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/engine/feeds"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/overview"
	overviewcalls "github.com/jterrazz/studio-station/src/overview/calls"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ports"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
)

const (
	thisMac = "jterrazz-macbook-pro"
	mini    = "jterrazz-mac-mini"
	gb      = int64(1) << 30
)

type steps struct {
	mu    sync.Mutex
	wakes map[time.Duration]chan struct{}
}

func newSteps() *steps {
	return &steps{wakes: map[time.Duration]chan struct{}{2 * time.Second: make(chan struct{}), 30 * time.Second: make(chan struct{})}}
}

func (s *steps) Sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	wake := s.wakes[d]
	s.mu.Unlock()
	select {
	case <-wake:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *steps) Now() time.Time { return time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC) }

func (s *steps) pass(t *testing.T, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case s.wakes[d] <- struct{}{}:
	case <-ctx.Done():
		t.Fatalf("nothing of the page waits %s", d)
	}
}

type world struct {
	mu       sync.Mutex
	infra    []machines.VM
	beside   []machines.VM
	here     []machines.VM
	added    []machines.HostView
	lent     accounts.Report
	updates  bool
	asked    map[string][]string
	activity map[string]uint64
}

func (w *world) machines(here bool) machines.AllMachines {
	w.mu.Lock()
	defer w.mu.Unlock()
	infra := w.infra[0]
	if len(w.infra) > 1 {
		w.infra = w.infra[1:]
	}
	if here {
		return machines.AllMachines{Hosts: []machines.HostView{{Name: thisMac, Here: true, Reachable: true, Daemon: machines.EngineView{Known: true, Running: true}, VMs: w.here}}}
	}
	return machines.AllMachines{Hosts: append([]machines.HostView{
		{Name: thisMac, Here: true, Reachable: true, Daemon: machines.EngineView{Known: true, Running: true}, VMs: w.here},
		{Name: mini, Reachable: true, Daemon: machines.EngineView{Known: true, Running: true}, VMs: append([]machines.VM{infra}, w.beside...)},
	}, w.added...)}
}

func (w *world) answer(name string, target machine.Ref, in any) any {
	w.mu.Lock()
	at := target.Name
	if at == "" {
		at = thisMac
	}
	w.asked[name] = append(w.asked[name], at)
	w.mu.Unlock()
	switch name {
	case machinescalls.List.Name:
		listed, _ := in.(machinescalls.ListIn)
		return machinescalls.ListOut{Fleet: w.machines(listed.Here)}
	case portscalls.List.Name:
		return ports.Listing{}
	case accountscalls.List.Name:
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.lent
	case overviewcalls.Read.Name:
		need := map[string]string{thisMac: "a sign-in expired", mini: "port 5000 is reachable from outside", "infra": "a container exited"}[at]
		needs := []overview.Need{{Machine: at, Section: "ports", Broken: true, Text: need}}
		w.mu.Lock()
		if at == thisMac && w.updates {
			needs = append(needs, overview.Need{Machine: at, Section: "apps", Subject: "updates", Text: "2 app updates", Call: "apps.update"})
		}
		w.mu.Unlock()
		return overviewcalls.ReadOut{Summary: overview.Summary{Head: overview.Head{Name: at}, Needs: needs}}
	case activitycalls.Sample.Name:
		memory := map[string]int64{thisMac: 16 * gb, mini: 64 * gb, "infra": 24 * gb}[at]
		w.mu.Lock()
		w.activity[at]++
		n := w.activity[at]
		w.mu.Unlock()
		return activity.Reading{Sample: activity.Sample{CPUBusy: n * 300, CPUTotal: n * 1000, MemUsed: memory / 2, MemTotal: memory}}
	}
	return nil
}

func (w *world) Invoke(_ context.Context, target machine.Ref, name string, in, out any, _ call.Progress) error {
	v := w.answer(name, target, in)
	if v == nil {
		return errors.New(name + " is not answered in this world")
	}
	reflect.ValueOf(out).Elem().Set(reflect.ValueOf(v))
	return nil
}

func (w *world) Watch(_ context.Context, target machine.Ref, name string, in any, _ call.Progress) (<-chan call.Update[any], error) {
	once := make(chan call.Update[any], 1)
	if v := w.answer(name, target, in); v != nil {
		once <- call.Update[any]{Value: v}
	}
	close(once)
	return once, nil
}

func (w *world) askedOf(name string) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.asked[name])
}

func infraAt(state machines.VMState, restarts int) machines.VM {
	return machines.VM{Name: "infra", Host: mini, Lifetime: machines.AlwaysOn, State: state, Want: "running", Guest: "connected", IP: "192.168.64.7", Check: "https://infra:6443/readyz", Device: "infra",
		Restarts: machines.Restarts{Count: restarts}, CheckStatus: machines.CheckStatus{State: machines.CheckPassing}}
}

func watch(t *testing.T, w *world, clock *steps, subject string) <-chan any {
	t.Helper()
	return watchWith(t, w, clock, engine.FeedInput{Subject: subject})
}

func watchWith(t *testing.T, w *world, clock *steps, in engine.FeedInput) <-chan any {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	o := feeds.Menubar{Calls: w, Clock: clock}
	return o.Machine()(ctx, in)
}

func until(t *testing.T, pages <-chan any, what string, ok func(page.Page) bool) page.Page {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		select {
		case v := <-pages:
			d, isPage := v.(page.Document)
			if !isPage {
				t.Fatalf("the feed sent %T, want a page", v)
			}
			if ok(d.Page) {
				return d.Page
			}
		case <-ctx.Done():
			t.Fatalf("the page never showed %s", what)
		}
	}
}

func find(items []page.Item, id string) *page.Row {
	for _, it := range items {
		if it.Row != nil && it.Row.ID == id {
			return it.Row
		}
		if it.Group != nil {
			if it.Group.Head.ID == id {
				return &it.Group.Head
			}
			if r := find(it.Group.Items, id); r != nil {
				return r
			}
		}
	}
	return nil
}

func row(p page.Page, section, id string) *page.Row {
	for _, s := range p.Sections {
		if s.ID == section {
			return find(s.Items, id)
		}
	}
	return nil
}

func needs(p page.Page) []string {
	var out []string
	for _, s := range p.Sections {
		if s.ID != "needs-you" {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil {
				out = append(out, it.Row.Label)
			}
		}
	}
	return out
}

func TestTheMacMinisPageOpenFromThisMacIsTheMinisOwnAnswer(t *testing.T) {
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, asked: map[string][]string{}, activity: map[string]uint64{}}
	p := until(t, watch(t, w, newSteps(), mini), "mac-mini's Needs you", func(p page.Page) bool { return len(needs(p)) > 0 })
	if got := needs(p); !slices.Equal(got, []string{"port 5000 is reachable from outside"}) {
		t.Errorf("mac-mini's page needs you for %q, want only the mini's own port 5000", got)
	}
	if got := w.askedOf(overviewcalls.Read.Name); !slices.Equal(got, []string{mini}) {
		t.Errorf("the overview was read on %q, want on mac-mini only", got)
	}
}

func TestInfrasPageFollowsItLiveWhileItRestarts(t *testing.T) {
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0), infraAt(machines.Stopped, 0), infraAt(machines.Running, 1)}, asked: map[string][]string{}, activity: map[string]uint64{}}
	clock := newSteps()
	pages := watch(t, w, clock, "infra")
	memory := func(p page.Page) page.Value {
		if r := row(p, "activity", "memory"); r != nil {
			return r.Cells[0]
		}
		return page.Value{}
	}
	p := until(t, pages, "infra running with its own memory", func(p page.Page) bool {
		return memory(p).Kind == page.KindRatio && row(p, "machine", "state").Mark == page.OK
	})
	if got := memory(p); got.Of != 24*gb {
		t.Errorf("infra's memory is %d of %d bytes, want infra's own 24 GB", got.N, got.Of)
	}
	if got := needs(p); len(got) != 0 {
		t.Errorf("infra's page draws Needs you %q", got)
	}
	clock.pass(t, 2*time.Second)
	until(t, pages, "infra's CPU after its second reading", func(p page.Page) bool { return row(p, "activity", "cpu").Cells[0].Kind == page.KindPercent })
	if got := w.askedOf(machinescalls.List.Name); len(got) != 1 {
		t.Errorf("two seconds re-read the machines %d times, want it read once, at the page's opening", len(got))
	}
	clock.pass(t, 30*time.Second)
	until(t, pages, "infra down while it restarts", func(p page.Page) bool { return row(p, "machine", "state").Mark == page.Broken })
	clock.pass(t, 30*time.Second)
	p = until(t, pages, "infra back", func(p page.Page) bool { return row(p, "machine", "state").Mark == page.OK })
	if r := row(p, "up", "restarts"); r == nil || r.Cells[0].Items[0].N != 1 {
		t.Errorf("infra back counts %+v restarts, want 1", r)
	}
	for _, name := range []string{activitycalls.Sample.Name, overviewcalls.Read.Name} {
		for _, at := range w.askedOf(name) {
			if at != "infra" {
				t.Errorf("%s was read on %s, want every read on infra, through its Mac", name, at)
			}
		}
	}
}

func TestTheSwitcherDrawsEachMacsServersThenWorkspacesAndRereadsEveryThirtySeconds(t *testing.T) {
	build := machines.VM{Name: "build", Host: mini, Lifetime: machines.Throwaway, State: machines.Running}
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0), infraAt(machines.Stopped, 0)}, beside: []machines.VM{build}, asked: map[string][]string{}, activity: map[string]uint64{}}
	clock := newSteps()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pages := feeds.Menubar{Calls: w, Clock: clock}.Switcher()(ctx, engine.FeedInput{})
	p := until(t, pages, "every Mac", func(p page.Page) bool { return row(p, "machines", "infra") != nil })
	var groups []string
	for _, it := range p.Sections[0].Items {
		if it.Group != nil && it.Group.Head.ID == mini {
			for _, g := range it.Group.Items {
				groups = append(groups, g.Group.Head.Label)
			}
		}
	}
	if !slices.Equal(groups, []string{"Servers", "Workspaces"}) {
		t.Errorf("mac-mini's groups are %q, want its Servers then its Workspaces", groups)
	}
	if r := row(p, "machines", "infra"); r.Mark != page.OK {
		t.Errorf("infra is drawn %q before it stopped", r.Mark)
	}
	clock.pass(t, 30*time.Second)
	until(t, pages, "infra down thirty seconds later", func(p page.Page) bool { return row(p, "machines", "infra").Mark == page.Broken })
}

func lentTo(vm string, providers ...string) accounts.Report {
	until := time.Date(2026, 11, 4, 6, 30, 0, 0, time.UTC)
	m := accounts.Machine{Name: vm, Kind: accounts.KindVM}
	for _, p := range providers {
		m.Accounts = append(m.Accounts, accounts.Account{Provider: p, Shared: &accounts.Lend{Name: p, Until: until}})
	}
	return accounts.Report{Machines: []accounts.Machine{m}}
}

func shared(p page.Page) []string {
	var out []string
	for _, s := range p.Sections {
		if s.ID != "shared" {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil {
				out = append(out, it.Row.ID)
			}
		}
	}
	return out
}

func TestAnAccountTakenBackLeavesLinux1sPageOnceTheCallReturns(t *testing.T) {
	linux := machines.VM{Name: "linux-1", Host: thisMac, Lifetime: machines.Permanent, State: machines.Running, Guest: "connected"}
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, here: []machines.VM{linux}, lent: lentTo("linux-1", "github", "claude"), asked: map[string][]string{}, activity: map[string]uint64{}}
	refresh := make(chan struct{}, 1)
	pages := watchWith(t, w, newSteps(), engine.FeedInput{Subject: "linux-1", Refresh: refresh})
	until(t, pages, "GitHub and Claude shared with linux-1", func(p page.Page) bool { return slices.Equal(shared(p), []string{"github", "claude"}) })
	w.mu.Lock()
	w.lent = lentTo("linux-1", "claude")
	w.mu.Unlock()
	refresh <- struct{}{}
	p := until(t, pages, "GitHub gone from linux-1's shared accounts", func(p page.Page) bool { return slices.Equal(shared(p), []string{"claude"}) })
	for _, a := range p.Actions {
		if a.ID == "take-back/github" {
			t.Errorf("linux-1's page still offers %q after GitHub was taken back", a.Label)
		}
	}
}

func TestAMacAddedReachesTheSwitcherOnceTheCallReturns(t *testing.T) {
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, asked: map[string][]string{}, activity: map[string]uint64{}}
	refresh := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pages := feeds.Menubar{Calls: w, Clock: newSteps()}.Switcher()(ctx, engine.FeedInput{Refresh: refresh})
	until(t, pages, "every Mac", func(p page.Page) bool { return row(p, "machines", mini) != nil })
	w.mu.Lock()
	w.added = []machines.HostView{{Name: "studio-1", Reachable: true, Daemon: machines.EngineView{Known: true, Running: true}}}
	w.mu.Unlock()
	refresh <- struct{}{}
	until(t, pages, "studio-1 in the switcher", func(p page.Page) bool { return row(p, "machines", "studio-1") != nil })
}

func TestAWorkspaceStartedOnThisMacReachesItsPageWhenTheEngineMoves(t *testing.T) {
	linux := machines.VM{Name: "linux-1", Host: thisMac, Lifetime: machines.Permanent, State: machines.Stopped}
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, here: []machines.VM{linux}, asked: map[string][]string{}, activity: map[string]uint64{}}
	changed := make(chan struct{}, 1)
	pages := watchWith(t, w, newSteps(), engine.FeedInput{Subject: "linux-1", Changed: changed})
	until(t, pages, "linux-1 stopped", func(p page.Page) bool { r := row(p, "machine", "state"); return r != nil && r.Mark != page.OK })
	w.mu.Lock()
	linux.State = machines.Running
	linux.Guest = "connected"
	w.here = []machines.VM{linux}
	w.mu.Unlock()
	changed <- struct{}{}
	until(t, pages, "linux-1 running", func(p page.Page) bool { r := row(p, "machine", "state"); return r != nil && r.Mark == page.OK })
}

func TestAWorkspaceCreatedFromTheMenuBarAppearsInHomeWhenTheEngineMoves(t *testing.T) {
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, asked: map[string][]string{}, activity: map[string]uint64{}}
	changed := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	o := feeds.Menubar{Calls: w, Clock: newSteps(), Report: func(awake.View) *awake.Report { return nil }}
	in := engine.FeedInput{Changed: changed}
	in.Power = nothing(in.Power)
	pages := o.Feed()(ctx, in)
	until(t, pages, "every Mac", func(p page.Page) bool { return row(p, "machines", mini) != nil })
	w.mu.Lock()
	w.here = []machines.VM{{Name: "linux-2", Host: thisMac, Lifetime: machines.Permanent, State: machines.Running, Guest: "connected"}}
	w.mu.Unlock()
	changed <- struct{}{}
	p := until(t, pages, "linux-2 in Home", func(p page.Page) bool { return row(p, "machines", "linux-2") != nil })
	if row(p, "machines", "infra") == nil {
		t.Error("Home lost mac-mini's infra when This Mac was read again")
	}
}

func TestUpdateAllLeavesHomesNeedsYouOnceTheCallReturns(t *testing.T) {
	w := &world{infra: []machines.VM{infraAt(machines.Running, 0)}, updates: true, asked: map[string][]string{}, activity: map[string]uint64{}}
	refresh := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	o := feeds.Menubar{Calls: w, Clock: newSteps(), Report: func(awake.View) *awake.Report { return nil }}
	in := engine.FeedInput{Refresh: refresh}
	in.Power = nothing(in.Power)
	pages := o.Feed()(ctx, in)
	was := until(t, pages, "2 app updates in Needs you", func(p page.Page) bool { return slices.Contains(needs(p), "2 app updates") })
	w.mu.Lock()
	w.updates = false
	w.mu.Unlock()
	refresh <- struct{}{}
	is := until(t, pages, "2 app updates gone from Needs you", func(p page.Page) bool { return len(needs(p)) > 0 && !slices.Contains(needs(p), "2 app updates") })
	if is.Badge.Needs != was.Badge.Needs-1 {
		t.Errorf("the badge went from %d to %d, want it down by one", was.Badge.Needs, is.Badge.Needs)
	}
}

func nothing[T any](func() T) func() T { return func() T { var zero T; return zero } }
