package menubar

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/page"
	pageawake "github.com/jterrazz/studio-station/src/pages/awake"
)

const (
	AwakeID page.ID = "menubar.awake"

	ActionKeep    = "keep:"
	ActionUntil   = "keep-until"
	ActionLid     = "lid"
	ActionBattery = "battery"
	ActionLetGo   = "let-sleep"
	ActionChange  = "change:"

	CallSettingsOff   = "settings.off"
	CallSettingsApply = "settings.apply"

	SectionThisMac = "this-mac"
	SectionGuards  = "guards"
	SectionOthers  = "other-macs"

	neverSleeps = "never-sleeps"
)

// value: the terms Keep awake for… offers, then how many whole hours
// Until… offers from the next one.
func terms() []time.Duration { return []time.Duration{time.Hour, 4 * time.Hour} }

const untilHours = 8

func Awake(f pageawake.AllMachines, ctx page.Context) page.Page {
	// invariant: SPEC §10.2 Awake, vision panel e: This Mac first, its term,
	// Lid closed and On battery only where its capabilities have them (E1),
	// its guards; then each other Mac by its mode, with Change…, a y/n that
	// names the servers it hosts. VMs are not listed: they follow their Mac.
	p := page.Page{ID: AwakeID, Command: "awake", Context: ctx}
	if f.Label == nil {
		self := ""
		for _, r := range f.Reports {
			if r.ThisMac {
				self = r.Mac
			}
		}
		f.Label = func(mac string) string { return page.MacName(mac, self) }
	}
	var others []page.Item
	for _, r := range f.Reports {
		if r.ThisMac {
			p.Sections = append(p.Sections, thisMac(r))
			if g := guards(r); g != nil {
				p.Sections = append(p.Sections, *g)
			}
			p.Actions = append(p.Actions, keeps(r, f.Now, f.Zone)...)
			continue
		}
		row := pageawake.Row(r)
		row.Short = label(f, r.Mac)
		others = append(others, page.Item{Row: &row})
		if a, ok := change(r, row.Short); ok {
			p.Actions = append(p.Actions, a)
		}
	}
	for _, s := range f.Silent {
		row := page.Row{ID: s.Mac, Label: s.Mac, Short: label(f, s.Mac), Mark: page.NotKnown, Reason: page.Unreachable, Cells: []page.Value{page.Text(s.Why)}}
		others = append(others, page.Item{Row: &row})
	}
	if len(others) > 0 {
		others = append(others, page.Item{Note: &page.Note{Text: "VMs are not listed: they follow their Mac."}})
		p.Sections = append(p.Sections, page.Section{ID: SectionOthers, Title: "Other Macs", Uncounted: true, Columns: awakeColumns(), Items: others})
	}
	for _, r := range f.Reports {
		if r.Engine == awake.EngineReading {
			p.State = page.Loading
		}
	}
	return page.Seal(p)
}

func label(f pageawake.AllMachines, mac string) string {
	if f.Label != nil {
		return f.Label(mac)
	}
	return mac
}

func awakeColumns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "mode", Kind: page.KindText, Keep: 100}}
}

func thisMac(r awake.Report) page.Section {
	row := pageawake.Row(r)
	row.Short = page.ThisMac
	return page.Section{ID: SectionThisMac, Title: page.ThisMac, Uncounted: true, Columns: awakeColumns(), Items: []page.Item{{Row: &row}}}
}

// value: the guards this Mac's capabilities allow, each with its reading
// (vision panel e: Temperature, Battery, the screen).
func guards(r awake.Report) *page.Section {
	if r.Engine == awake.EngineReading {
		return nil
	}
	var facts []page.Fact
	if r.Caps.Thermal {
		facts = append(facts, page.Fact{Key: "Temperature", Value: page.Text(temperature(r.Reading.Thermal))})
	}
	if r.Caps.Battery {
		facts = append(facts, page.Fact{Key: "Battery", Value: page.Text(strconv.Itoa(r.Reading.Percent) + " %")})
	}
	facts = append(facts, page.Fact{Key: "The screen", Value: page.Texts("always turns off", "protects the display")})
	items := make([]page.Item, 0, len(facts)+1)
	for i := range facts {
		items = append(items, page.Item{Fact: &facts[i]})
	}
	if r.Caps.Thermal {
		items = append(items, page.Item{Note: &page.Note{Text: "Awake stops by itself if the Mac gets too hot."}})
	}
	return &page.Section{ID: SectionGuards, Title: "Guards", Uncounted: true, Columns: awakeColumns(), Items: items}
}

// value: the Mac's thermal level in the menu bar's plain word: Normal
// for the level macOS calls nominal.
func temperature(n int) string {
	if w := awake.Level(n); w != "nominal" {
		return w
	}
	return "normal"
}

func nanos(d time.Duration) string { return strconv.FormatInt(int64(d), 10) }

// value: an end as the call's input takes it, a JSON time.
func moment(t time.Time) string { return `"` + t.UTC().Format(time.RFC3339) + `"` }

func flags(lid, battery bool) map[string]string {
	return map[string]string{"Lid": strconv.FormatBool(lid), "Battery": strconv.FormatBool(battery), "By": "the menu bar"}
}

// invariant: a keep-awake keeps the switches as they are; a switch keeps
// the end the keep-awake holds until, or starts the 4 h one. While held, Until… says
// the time it holds until (vision panel e: `Until 18:00`), and the term it
// was taken for is the one picked, else Until.
func keeps(r awake.Report, now time.Time, zone *time.Location) []page.Action {
	var lid, battery bool
	held := r.Session != nil && r.Mode() == awake.AwakeUntil
	if held {
		lid, battery = r.Session.Lid, r.Session.Battery
	}
	rows := []string{r.Mac}
	var out []page.Action
	picked := false
	for _, t := range terms() {
		in := flags(lid, battery)
		in["For"] = nanos(t)
		on := held && asked(*r.Session) == t
		picked = picked || on
		out = append(out, page.Action{ID: ActionKeep + strings.TrimSuffix(t.String(), "0m0s"), Label: pageTerm(t), On: page.OnRow, Rows: rows, Priority: 1, Mutates: true, Picked: on, Run: &page.Run{Call: CallKeepAwake, In: in}})
	}
	until := "Until"
	if held {
		if zone == nil {
			zone = time.UTC
		}
		until += " " + r.Session.Until.In(zone).Format("15:04")
	}
	out = append(out, page.Action{ID: ActionUntil, Label: until, On: page.OnRow, Rows: rows, Priority: 1, Mutates: true, Picked: held && !picked,
		Form: &page.Form{Title: "Keep awake until", Fields: []page.Field{untilField(now, zone)}}, Run: &page.Run{Call: CallKeepAwake, In: flags(lid, battery)}})
	term := func(in map[string]string) map[string]string {
		if held {
			in["Until"] = moment(r.Session.Until)
		} else {
			in["For"] = nanos(awake.DefaultFor)
		}
		return in
	}
	if r.Caps.Lid {
		in := term(flags(!lid, battery))
		out = append(out, page.Action{ID: ActionLid, Label: "Lid closed", On: page.OnRow, Rows: rows, Priority: 2, Mutates: true, Run: &page.Run{Call: CallKeepAwake, In: in}})
	}
	if r.Caps.Battery {
		in := term(flags(lid, !battery))
		out = append(out, page.Action{ID: ActionBattery, Label: "On battery", On: page.OnRow, Rows: rows, Priority: 2, Mutates: true, Run: &page.Run{Call: CallKeepAwake, In: in}})
	}
	if held {
		out = append(out, page.Action{ID: ActionLetGo, Label: "Let it sleep", On: page.OnRow, Rows: rows, Priority: 3, Mutates: true, Run: &page.Run{Call: CallAwakeOff}})
	}
	return out
}

// value: the term a session was asked for: its own word, else its span
// from its start.
func asked(s awake.Session) time.Duration {
	if s.For > 0 {
		return s.For.Round(time.Minute)
	}
	return s.Until.Sub(s.StartedAt).Round(time.Minute)
}

func pageTerm(d time.Duration) string { return strconv.Itoa(int(d.Hours())) + " h" }

func untilField(now time.Time, zone *time.Location) page.Field {
	if zone == nil {
		zone = time.UTC
	}
	next := now.In(zone).Truncate(time.Hour).Add(time.Hour)
	opts := make([]page.Option, 0, untilHours)
	for i := range untilHours {
		at := next.Add(time.Duration(i) * time.Hour)
		opts = append(opts, page.Option{Value: moment(at), Label: at.Format("15:04")})
	}
	return page.Field{Key: "Until", Label: "Until", Kind: page.FieldSelect, Options: opts, Default: opts[len(opts)/2].Value}
}

// invariant: AW-21: Change… on a Mac that never sleeps lets it sleep again,
// a y/n that names each server it hosts, which pauses while it sleeps; on
// one that sleeps it keeps it awake for good. It runs on that Mac.
func change(r awake.Report, name string) (page.Action, bool) {
	if r.Engine == awake.EngineReading || (r.Engine != "" && r.Engine != awake.EngineRunning) {
		return page.Action{}, false
	}
	run := &page.Run{Target: &page.Target{Name: r.Mac}, In: map[string]string{"Names": `["` + neverSleeps + `"]`, "Confirm": "yes"}}
	a := page.Action{ID: ActionChange + r.Mac, Label: "Change", On: page.OnRow, Rows: []string{r.Mac}, Priority: 2, Mutates: true, Run: run}
	if r.Mode() == awake.NeverSleeps {
		run.Call = CallSettingsOff
		lines := make([]page.Value, 0, len(r.Servers))
		for _, s := range r.Servers {
			lines = append(lines, page.Text("It hosts "+s+" (server): "+s+" pauses while "+name+" sleeps."))
		}
		a.Confirm = &page.Confirm{Question: "Let " + name + " sleep again?", Lines: lines, Yes: "Allow sleep", No: "Keep it awake", Danger: len(r.Servers) > 0}
		return a, true
	}
	run.Call = CallSettingsApply
	a.Confirm = &page.Confirm{Question: "Keep " + name + " awake for good?", Lines: []page.Value{page.Text("Never sleeps: the always-on Mac policy, until you change it again.")}, Yes: "Never sleep", No: "Cancel"}
	return a, true
}
