package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/page"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
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
		h.VMs = append(h.VMs, machines.VM{Name: n, Kind: catalog.Linux, Host: "macbook", Lifetime: machines.Permanent, State: machines.Running})
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
	busy, _ := v.Act(func() tea.Msg { return nil })
	if _, _, ev := busy.Update(key("s")); ev != nil {
		t.Errorf("a busy tab reported %+v", ev)
	}
}

// value: what the program does with an action's ExecMsg: run it, then its Done.
func execute(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg, ok := cmd().(tab.ExecMsg)
	if !ok {
		t.Fatal("the action did not release the terminal")
	}
	_ = msg.Command()
	return msg.Done(nil)
}

// value: a restart whose call asks yes or no, recording each answer it is given.
func restart(answers *[]string) func(string) (tab.Status, error) {
	return func(answer string) (tab.Status, error) {
		*answers = append(*answers, answer)
		if err := confirm.Require(confirm.YesNo, "macbook", answer); err != nil {
			return tab.Status{}, err
		}
		return tab.Status{Text: "restarted macbook"}, nil
	}
}

func TestAnActionItsCallDoesNotConfirmRunsWithoutAQuestion(t *testing.T) {
	v := view(fleetOf())
	var answers []string
	release := tab.Confirmed("fleet", pagefleet.ForgetConfirm("macbook"), func(answer string) (tab.Status, error) {
		answers = append(answers, answer)
		return tab.Status{Text: "released"}, nil
	})
	v, _ = v.Act(release)
	msg := execute(t, release)
	if done, ok := msg.(tab.Done); !ok || done.Status.Text != "released" {
		t.Fatalf("the action ended with %+v, want its Done", msg)
	}
	if v, _, _ = v.Update(msg); v.Modal() || len(answers) != 1 || answers[0] != "" {
		t.Errorf("an action whose call asks nothing put a question (modal %v) or ran answered: %q", v.Modal(), answers)
	}
}

func TestAConfirmationTheCallAsksIsPutAndYesRunsItAgainAnswered(t *testing.T) {
	v := view(fleetOf())
	var answers []string
	run := tab.Confirmed("fleet", pagefleet.RestartConfirm("macbook", false), restart(&answers))
	v, _ = v.Act(run)
	v, _, _ = v.Update(execute(t, run))
	if !v.Modal() {
		t.Fatal("the call asked a confirmation and no question was put")
	}
	v, cmd, _ := v.Update(key("y"))
	_, cmd, _ = v.Update(cmd())
	done, ok := execute(t, cmd).(tab.Done)
	if !ok || done.Err != nil || done.Status.Text != "restarted macbook" {
		t.Fatalf("yes ended with %+v, want the restart done", done)
	}
	if len(answers) != 2 || answers[0] != "" || answers[1] != confirm.Yes {
		t.Errorf("the call was given %q, want first nothing, then yes", answers)
	}
}

func TestAHostRestartAsksItsName(t *testing.T) {
	v := view(fleetOf())
	var answers []string
	run := tab.Confirmed("fleet", pagefleet.RestartConfirm("macbook", true), restart(&answers))
	v, _ = v.Act(run)
	v, _, _ = v.Update(execute(t, run))
	if v.Browser.Focus() == 0 || !v.Modal() {
		t.Fatal("the restart opened no modal")
	}
	_, cmd, _ := v.Update(key("y"))
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, started := msg.(tab.ExecMsg); started {
				t.Error("y confirmed a restart that asks the name")
			}
		}
	}
	if len(answers) != 1 {
		t.Errorf("the restart ran %d times, want once, unanswered", len(answers))
	}
}

func TestAHiddenTabShowsTheAnswerItHeldOnceShown(t *testing.T) {
	here := page.Context{Kind: "host"}
	answer := fleetOf("api")
	source := func(context.Context, bool) (<-chan pagefleet.Reading, error) {
		ch := make(chan pagefleet.Reading, 1)
		ch <- pagefleet.Reading{Machines: &answer}
		return ch, nil
	}
	v := tab.NewView("fleet", "Fleet", "hosts", here, source, func(r pagefleet.Reading) page.Page { return pagefleet.Tab(r, here) }, nil)
	v, _, _ = v.Update(tab.Shown{Visible: false})
	v, _, _ = v.Update(v.Init()())
	if strings.Contains(v.Body(100, 20), "api") {
		t.Fatal("a hidden tab laid out its answer")
	}
	v, _, _ = v.Update(tab.Shown{Visible: true})
	if !strings.Contains(v.Body(100, 20), "api") {
		t.Errorf("the shown tab lost the answer it held:\n%s", v.Body(100, 20))
	}
}
