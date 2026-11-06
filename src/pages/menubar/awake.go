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

func Awake(f pageawake.AllMachines, here page.Context) page.Page {
	// invariant: SPEC §10.2 Awake, vision panel e: This Mac first, its term,
	// Lid closed and On battery only where its capabilities have them (E1),
	// its guards; then each other Mac by its mode, with Change…, a y/n that
	// names the servers it hosts. VMs are not listed: they follow their Mac.
	p := page.Page{ID: AwakeID, Command: "awake", Context: here}
	var others []page.Item
	for _, r := range f.Reports {
		if r.Here {
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
		if r.Daemon == awake.EngineReading {
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
	if r.Daemon == awake.EngineReading {
		return nil
	}
	var facts []page.Fact
	if r.Caps.Thermal {
		facts = append(facts, page.Fact{Key: "Temperature", Value: page.Text(awake.Level(r.Reading.Thermal))})
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

func nanos(d time.Duration) string { return strconv.FormatInt(int64(d), 10) }

func flags(lid, battery bool) map[string]string {
	return map[string]string{"Lid": strconv.FormatBool(lid), "Battery": strconv.FormatBool(battery), "By": "the menu bar"}
}

// invariant: a keep-awake keeps the switches as they are; a switch keeps
// the term that is left, or starts the 4 h one.
func keeps(r awake.Report, now time.Time, zone *time.Location) []page.Action {
	var lid, battery bool
	left := 4 * time.Hour
	held := r.Session != nil && r.Mode() == awake.AwakeUntil
	if held {
		lid, battery, left = r.Session.Lid, r.Session.Battery, r.Session.Until.Sub(now).Round(time.Minute)
	}
	rows := []string{r.Mac}
	var out []page.Action
	for _, t := range terms() {
		in := flags(lid, battery)
		in["For"] = nanos(t)
		out = append(out, page.Action{ID: ActionKeep + strings.TrimSuffix(t.String(), "0m0s"), Label: pageTerm(t), On: page.OnRow, Rows: rows, Priority: 1, Mutates: true, Run: &page.Run{Call: CallAwake, In: in}})
	}
	out = append(out, page.Action{ID: ActionUntil, Label: "Until", On: page.OnRow, Rows: rows, Priority: 1, Mutates: true,
		Form: &page.Form{Title: "Keep awake until", Fields: []page.Field{untilField(now, zone)}}, Run: &page.Run{Call: CallAwake, In: flags(lid, battery)}})
	if r.Caps.Lid {
		in := flags(!lid, battery)
		in["For"] = nanos(left)
		out = append(out, page.Action{ID: ActionLid, Label: "Lid closed", On: page.OnRow, Rows: rows, Priority: 2, Mutates: true, Run: &page.Run{Call: CallAwake, In: in}})
	}
	if r.Caps.Battery {
		in := flags(lid, !battery)
		in["For"] = nanos(left)
		out = append(out, page.Action{ID: ActionBattery, Label: "On battery", On: page.OnRow, Rows: rows, Priority: 2, Mutates: true, Run: &page.Run{Call: CallAwake, In: in}})
	}
	if held {
		out = append(out, page.Action{ID: ActionLetGo, Label: "Let it sleep", On: page.OnRow, Rows: rows, Priority: 3, Mutates: true, Run: &page.Run{Call: CallSleep}})
	}
	return out
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
		opts = append(opts, page.Option{Value: nanos(at.Sub(now)), Label: at.Format("15:04")})
	}
	return page.Field{Key: "For", Label: "Until", Kind: page.FieldSelect, Options: opts, Default: opts[len(opts)/2].Value}
}

// invariant: AW-21: Change… on a Mac that never sleeps lets it sleep again,
// a y/n that names each server it hosts, which pauses while it sleeps; on
// one that sleeps it keeps it awake for good. It runs on that Mac.
func change(r awake.Report, name string) (page.Action, bool) {
	if r.Daemon == awake.EngineReading || (r.Daemon != "" && r.Daemon != awake.EngineRunning) {
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
