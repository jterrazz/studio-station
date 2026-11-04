package calls

import (
	"context"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

const (
	Unknown           fail.Code = "setting.unknown"
	NotForThisMachine fail.Code = "setting.not-for-this-machine"
	NeedsTerminal     fail.Code = "setting.needs-terminal"
	// value: a checked setting named to apply: station reads it, never sets it.
	IsChecked fail.Code = "setting.checked"
)

type Deps struct {
	Settings Settings
	Appliers contract.Appliers
	Applied  contract.AppliedStore
	OffHere  contract.OffHerePrefs
	Now      func() time.Time
	// value: the values a setting asks, from the owner at a terminal; nil
	// where nobody is there to answer.
	Ask       Ask
	Shortcuts Shortcuts
	// value: the script the ~/.zshrc block evaluates.
	Shell Shell
	Kind  machine.Kind
	// value: the facts a checked setting is judged on, and what this machine
	// reports it has (a battery makes it a laptop).
	Facts contract.Inspector
	Caps  machine.Capabilities
}

type Shell interface {
	Zsh(ctx context.Context, completion string) (string, error)
}

type Settings interface {
	Items() []catalog.ConfigItem
	Lives(it catalog.ConfigItem) bool
	// value: the app a setting needs that is not installed here; empty when
	// it needs none or has it.
	Needs(it catalog.ConfigItem) string
	// invariant: Admit refuses, before any effect, a setting this run cannot
	// apply (root).
	Admit(name string, interactive bool) error
	// invariant: Apply admits the setting (root), merges its default values
	// with the ones given, and writes it through its Applier.
	Apply(ctx context.Context, name string, values map[string]string, interactive bool) (settings.Change, error)
}

type Ask interface {
	Values(item catalog.ConfigItem) (map[string]string, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{
		call.BindWatch(List, d), call.Bind(Show, d), call.Bind(Diff, d), call.Bind(Apply, d), call.Bind(Off, d),
		call.Bind(Git, d), call.Bind(Containers, d), call.Bind(Zsh, d),
	}
}

func asks(it catalog.ConfigItem) bool { return len(it.Inputs) > 0 || it.Interactive }

// invariant: a setting is named by its catalogue name; one that lives on
// no machine of this kind is refused before any effect.
func find(d Deps, name string, living bool) (catalog.ConfigItem, error) {
	for _, it := range d.Settings.Items() {
		if it.Name != name {
			continue
		}
		if living && !d.Settings.Lives(it) {
			return it, fail.New(fail.NotForThisMachine, NotForThisMachine, name, map[string]string{"why": settings.PlaceReason(it.Places)})
		}
		return it, nil
	}
	return catalog.ConfigItem{}, fail.New(fail.Refused, Unknown, name, nil)
}

func offHere(d Deps) map[string]bool {
	names, _ := d.OffHere.Get()
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

func record(ctx context.Context, d Deps, name string) *settings.Applied {
	a, err := d.Applied.Get(ctx, name)
	if err != nil {
		return nil
	}
	return &a
}

// value: one setting judged now: observed through its Applier, beside what
// station recorded and the owner's Off here.
func judge(ctx context.Context, d Deps, it catalog.ConfigItem, off map[string]bool) (settings.Row, settings.Observation) {
	lives := d.Settings.Lives(it)
	var now settings.Observation
	if lives && !off[it.Name] {
		if a, ok := d.Appliers.For(it); ok {
			o, err := a.Observe(ctx, it)
			if err != nil {
				o = settings.Observation{State: catalog.StateBroken, Detail: err.Error()}
			}
			now = o
		}
	}
	row := settings.JudgeSet(it, lives, record(ctx, d, it.Name), now, off[it.Name])
	if row.State == settings.StateNotApplied {
		row.Needs = d.Settings.Needs(it)
	}
	return row, now
}

// value: one checked setting judged now: its facts read, each in its
// budget, beside the owner's Off here and what this Mac should have.
func judgeChecked(ctx context.Context, d Deps, it catalog.CheckedSetting, off map[string]bool, expect settings.Expectation) settings.Row {
	lives := d.Settings.Lives(catalog.ConfigItem{Name: it.Name, Places: it.Places})
	facts := map[catalog.Fact]settings.Fact{}
	if lives && !off[it.Name] && d.Facts != nil {
		for _, f := range it.Facts {
			facts[f] = readFact(ctx, d.Facts, f, it.Budget)
		}
	}
	return settings.JudgeChecked(it, lives, facts, expect, off[it.Name])
}

func readFact(ctx context.Context, i contract.Inspector, f catalog.Fact, budget time.Duration) settings.Fact {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	got, err := i.Read(ctx, f)
	if err != nil {
		return settings.Fact{State: catalog.StateUnknown, Detail: "no answer in " + strconv.Itoa(int(budget/time.Second)) + " s"}
	}
	return got
}

// invariant: an always-on Mac is one whose Never sleeps is applied, as its
// own row reads it.
func expectation(ctx context.Context, d Deps, off map[string]bool) settings.Expectation {
	alwaysOn := false
	for _, it := range d.Settings.Items() {
		if it.Effect == catalog.ProcPower {
			row, _ := judge(ctx, d, it, off)
			alwaysOn = row.State == settings.StateApplied
		}
	}
	return settings.Expect(d.Caps, alwaysOn, d.Now())
}
