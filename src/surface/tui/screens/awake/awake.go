package awake

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	pageawake "github.com/jterrazz/studio-station/src/pages/awake"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

const (
	lid     = "lid"
	battery = "battery"
	on      = "on"
)

type Screen struct{}

func (Screen) Live() time.Duration {
	// invariant: SPEC §9.4's Awake keys act on the Mac under the cursor; the
	// page is read again every second while shown, so a guard's end shows
	// at once (AW-20).
	return time.Second
}

func (Screen) Words(_ screen.Machine, words []string, _ screen.State) []string { return words }

func (Screen) Title(v screen.View) string {
	n := 0
	for _, s := range v.Page.Sections {
		if s.ID == "awake" {
			n = len(s.Items)
		}
	}
	return "Awake" + ui.Sep + "every Mac" + ui.Sep + strconv.Itoa(n)
}

func (Screen) Skips(_ screen.Machine, s page.Section) bool {
	// value: the cursor walks the Macs, never This Mac's readings.
	return s.ID != "awake"
}

func (Screen) Lines(v screen.View) ([]string, bool) {
	// invariant: vision screen 7: a block per Mac, this one first, its state
	// then what its keys do on it; the policy's Mac says what set it and the
	// servers that pause while it sleeps.
	a := ui.Awake{Intro: "Every Mac, this one first. VMs follow their Mac and are not listed.",
		Ending: "From here or from any Mac: the same controls, sent over " + catalog.TailnetProduct + ".",
		Cursor: ui.Cursor{Key: v.Cursor, Focused: v.Focused, Dark: v.Dark, NoColor: v.NoColor, Width: v.Width, Room: v.Room}}
	for _, s := range v.Page.Sections {
		if s.ID != "awake" {
			continue
		}
		for _, it := range s.Items {
			if it.Row == nil || (v.Match != nil && !v.Match(*it.Row)) {
				continue
			}
			a.Macs = append(a.Macs, block(v, s, *it.Row))
		}
	}
	return a.Lines(), true
}

func block(v screen.View, s page.Section, r page.Row) ui.AwakeMac {
	t := of(v.Page, r)
	m := ui.AwakeMac{Key: s.ID + "/" + r.ID, Title: []string{t.label}, Mark: r.Mark, State: t.words}
	if r.ID != t.label {
		m.Title = append(m.Title, r.ID)
	}
	if t.thisMac {
		m.Title = append(m.Title, "you are here")
	} else {
		m.Title = append(m.Title, "over "+catalog.TailnetProduct)
	}
	switch {
	case r.Mark == page.Awaiting || r.Mark == page.NotKnown || r.Reason == page.Stopped:
	case len(t.words) > 0 && t.words[0] == "Never sleeps":
		m.Lines = policy(t)
	default:
		m.Lines = controls(v, t)
	}
	return m
}

func policy(t target) []ui.AwakeLine {
	var out []ui.AwakeLine
	var servers []string
	for _, d := range t.detail {
		if slept, ok := strings.CutPrefix(d, "last slept: "); ok {
			out = append(out, ui.AwakeLine{Label: "Last slept", Text: slept})
		}
		if h, ok := strings.CutPrefix(d, "hosts "); ok {
			for _, one := range strings.Split(h, ", ") {
				name, _, _ := strings.Cut(one, " ")
				servers = append(servers, name)
			}
		}
	}
	out = append(out, ui.AwakeLine{Label: "Set by", Text: strings.Join([]string{section.Settings.Title(), "Always-on Mac", "Never sleeps"}, ui.Crumb)})
	asks := "asks: y/n"
	if len(servers) > 0 {
		kind, pause := "Server", "it pauses"
		if len(servers) > 1 {
			kind, pause = "Servers", "they pause"
		}
		out = append(out, ui.AwakeLine{Label: "Hosts", Text: strings.Join(servers, ", ") + "  " + kind + ui.Sep + pause + " if " + t.label + " sleeps"})
		asks += ", names " + strings.Join(servers, ", ")
	}
	return append(out, ui.AwakeLine{Keys: []ui.Pair{{Label: "c", Text: "Change" + ui.Ellipsis}}, Text: "keep the policy, or allow sleep (" + asks + ")"})
}

func controls(v screen.View, t target) []ui.AwakeLine {
	until := "until" + ui.Ellipsis
	if at := t.until(); at != "" {
		until = "until " + at + " (on)"
	}
	out := []ui.AwakeLine{{Label: "For", Keys: []ui.Pair{{Label: "1", Text: "1 h"}, {Label: "4", Text: "4 h"}, {Label: "u", Text: until}, {Label: "o", Text: "off"}}}}
	st := v.State
	if t.until() != "" {
		st = st.With(lid, onOff(t.held("lid closed"))).With(battery, onOff(t.held("on battery allowed")))
	}
	toggles := ui.AwakeLine{Text: "toggle"}
	if t.lid {
		toggles.Toggles = append(toggles.Toggles, ui.Toggle{Label: "Lid closed", On: st[lid] == on})
		toggles.Keys = append(toggles.Keys, ui.Pair{Label: "l"})
	}
	if t.battery {
		toggles.Toggles = append(toggles.Toggles, ui.Toggle{Label: "On battery", On: st[battery] == on})
		toggles.Keys = append(toggles.Keys, ui.Pair{Label: "b"})
	}
	if len(toggles.Toggles) > 0 {
		out = append(out, toggles)
	}
	if t.thisMac {
		if g := guards(v.Page); g != "" {
			out = append(out, ui.AwakeLine{Label: "Guards", Text: g})
		}
	}
	return out
}

// value: what ends a keep-awake on This Mac, from its readings: its
// temperature, its battery, and the screen, which still turns off.
func guards(p page.Page) string {
	var temperature, charge string
	for _, s := range p.Sections {
		if s.ID != "reading" {
			continue
		}
		for _, it := range s.Items {
			f := it.Fact
			switch {
			case f == nil:
			case f.Key == "thermal":
				temperature = "temperature " + ui.Plain(f.Value)
			case f.Key == "power source" && len(f.Value.Items) > 1:
				charge = "battery " + ui.Plain(f.Value.Items[1])
			}
		}
	}
	words := slices.DeleteFunc([]string{temperature, charge}, func(w string) bool { return w == "" })
	if len(words) == 0 {
		return ""
	}
	return strings.Join(append(words, "the screen still turns off"), ui.Sep)
}

func (Screen) Keys(v screen.View) []ui.Binding {
	t, ok := mac(v)
	if !ok {
		return nil
	}
	keys := []ui.Binding{screen.Bind("1", "1 h"), screen.Bind("4", "4 h"), screen.Bind("u", "until"+ui.Ellipsis), screen.Bind("o", "off")}
	if t.lid {
		keys = append(keys, screen.Bind("l", "lid"))
	}
	if t.battery {
		keys = append(keys, screen.Bind("b", "battery"))
	}
	return append(keys, screen.Bind("c", "change the policy"+ui.Ellipsis))
}

type target struct {
	name, label string
	thisMac     bool
	// value: whether the Mac reports a lid and a battery (E1), as the page's
	// lid and battery actions name its row.
	lid, battery bool
	words        []string
	detail       []string
}

func mac(v screen.View) (target, bool) {
	if !v.HasRow || v.Of.ID != "awake" {
		return target{}, false
	}
	return of(v.Page, v.Row), true
}

func of(p page.Page, r page.Row) target {
	t := target{name: r.ID, label: cmp.Or(r.Short, r.Label)}
	t.thisMac = t.label == "This Mac"
	t.lid, t.battery = offers(p, pageawake.KeyLid, t.name), offers(p, pageawake.KeyBattery, t.name)
	if len(r.Cells) > 0 {
		for _, w := range r.Cells[0].Items {
			t.words = append(t.words, ui.Plain(w))
		}
		if len(t.words) == 0 {
			t.words = []string{ui.Plain(r.Cells[0])}
		}
	}
	for _, w := range r.Detail.Items {
		t.detail = append(t.detail, ui.Plain(w))
	}
	if len(t.detail) == 0 && !r.Detail.IsNone() {
		t.detail = []string{ui.Plain(r.Detail)}
	}
	return t
}

func offers(p page.Page, key, row string) bool {
	for _, a := range p.Actions {
		if a.Key == key && slices.Contains(a.Rows, row) {
			return true
		}
	}
	return false
}

func (t target) on() []string {
	if t.thisMac {
		return nil
	}
	return []string{"--on", t.label}
}

// value: the end of the keep-awake the Mac holds, as its page says it
// (`Awake until 14:00`); empty when it holds none.
func (t target) until() string {
	if len(t.words) == 0 {
		return ""
	}
	at, _ := strings.CutPrefix(t.words[0], "Awake until ")
	if at == t.words[0] {
		return ""
	}
	return at
}

func (t target) held(word string) bool { return slices.Contains(t.words, word) }

func (t target) flags(st screen.State) []string {
	var out []string
	if t.lid && st[lid] == on {
		out = append(out, "--lid")
	}
	if t.battery && st[battery] == on {
		out = append(out, "--battery")
	}
	return out
}

func (t target) keep(st screen.State, how []string, said string) *screen.Call {
	words := append(append(append([]string{"awake"}, how...), t.flags(st)...), t.on()...)
	return &screen.Call{Words: words, Doing: "Keeping " + t.label + " awake " + said, Done: t.label + " stays awake " + said}
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	t, ok := mac(v)
	if !ok {
		return screen.Effect{}, false
	}
	st := v.State
	switch key {
	case "1":
		return screen.Effect{Call: t.keep(st, []string{"1h"}, "for 1 h")}, true
	case "4":
		return screen.Effect{Call: t.keep(st, []string{"4h"}, "for 4 h")}, true
	case "u":
		c := t.keep(st, nil, "until then")
		c.Form = &screen.Form{Title: "Keep " + t.label + " awake until", Fields: []screen.Field{{Label: "Until", Value: "18:00", Note: "a time today, or tomorrow once it has passed"}},
			Words: func(answers []string) []string {
				return append(append([]string{"awake", "until", strings.TrimSpace(answers[0])}, t.flags(st)...), t.on()...)
			}}
		return screen.Effect{Call: c}, true
	case "o":
		return screen.Effect{Call: &screen.Call{Words: append([]string{"awake", "off"}, t.on()...), Doing: "Letting " + t.label + " sleep", Done: t.label + " sleeps normally again"}}, true
	case "l", "b":
		if (key == "l" && !t.lid) || (key == "b" && !t.battery) {
			return screen.Effect{}, false
		}
		return toggle(t, st, map[string]string{"l": lid, "b": battery}[key]), true
	case "c":
		return change(t), true
	}
	return screen.Effect{}, false
}

// invariant: lid and battery are the keep-awake's own: while one is held
// the toggle holds it again to the same end with the other flags, else it
// is kept for the next one.
func toggle(t target, st screen.State, which string) screen.Effect {
	words := map[string]string{lid: "lid closed", battery: "on battery allowed"}
	now := st[which] == on
	if until := t.until(); until != "" {
		st = st.With(lid, onOff(t.held(words[lid]))).With(battery, onOff(t.held(words[battery])))
		now = st[which] == on
	}
	st = st.With(which, onOff(!now))
	said := map[string]string{lid: "Lid closed", battery: "On battery"}[which] + " " + map[bool]string{true: "on", false: "off"}[!now]
	if until := t.until(); until != "" {
		c := t.keep(st, []string{"until", until}, "until "+until)
		c.Done = t.label + ": " + said
		return screen.Effect{State: st, Call: c}
	}
	return screen.Effect{State: st, Say: said + " for the next keep-awake"}
}

func onOff(b bool) string {
	if b {
		return on
	}
	return ""
}

// invariant: SPEC §9.4, AW-19: changing a Mac's policy asks yes or no, and
// names the servers it hosts, which pause while it sleeps.
func change(t target) screen.Effect {
	var hosts []string
	for _, d := range t.detail {
		if h, ok := strings.CutPrefix(d, "hosts "); ok {
			hosts = append(hosts, h)
		}
	}
	on := []string{"--on", t.label}
	if t.thisMac {
		on = nil
	}
	if len(t.words) > 0 && t.words[0] == "Never sleeps" {
		ask := &screen.Ask{Question: "Let " + t.label + " sleep again?", Yes: "allow sleep"}
		for _, h := range hosts {
			name, _, _ := strings.Cut(h, " ")
			ask.Lines = append(ask.Lines, "It hosts "+h+": "+name+" pauses while "+t.label+" sleeps.")
		}
		return screen.Effect{Call: &screen.Call{Words: append([]string{"settings", "off", "never-sleeps", "--yes"}, on...), Ask: ask,
			Doing: "Letting " + t.label + " sleep again", Done: t.label + " sleeps normally"}}
	}
	ask := &screen.Ask{Question: "Keep " + t.label + " awake for good?", Lines: []string{"Never sleeps: the always-on Mac policy, until you change it again."}, Yes: "never sleep"}
	return screen.Effect{Call: &screen.Call{Words: append([]string{"settings", "apply", "never-sleeps", "--yes"}, on...), Ask: ask,
		Doing: "Keeping " + t.label + " awake for good", Done: t.label + " never sleeps"}}
}
