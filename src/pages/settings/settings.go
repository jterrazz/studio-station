package settings

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
	"github.com/jterrazz/studio-station/src/settings"
)

const (
	ID       = "settings"
	KeyApply = "a"
	KeyOff   = "o"
	KeyDiff  = "d"
)

func columns() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "state", Kind: page.KindText, Keep: 60}}
}

// value: how long ago, in its largest unit alone (`2 d ago`).
func ago(since, now time.Time) string {
	d := now.Sub(since)
	for _, u := range []struct {
		n    time.Duration
		name string
	}{{24 * time.Hour, "d"}, {time.Hour, "h"}, {time.Minute, "min"}} {
		if d >= u.n {
			return strconv.Itoa(int(d/u.n)) + " " + u.name + " ago"
		}
	}
	return "just now"
}

func Row(r settings.Row, now time.Time) page.Row {
	// value: one setting as a row: its mark, its state in words, where it
	// lives, and the next step.
	name := r.Item().Name
	row := page.Row{ID: name, Label: name}
	switch r.State {
	case settings.StateApplied:
		row.Mark, row.Cells = page.OK, []page.Value{page.Texts("applied", r.Where)}
	case settings.StateChangedSince:
		row.Mark, row.Reason = page.Attention, page.Differs
		row.Cells = []page.Value{page.Text("changed since " + ago(r.Since, now))}
		row.Hint = page.KeyHint("station settings diff "+name, KeyDiff)
	case settings.StateNotApplied:
		row.Mark, row.Cells = page.Absent, []page.Value{page.Text("not applied")}
		row.Hint = page.KeyHint("station settings apply "+name, KeyApply)
		if r.Needs != "" {
			row.Reason = page.Missing
			row.Cells = []page.Value{page.Texts("not applied", "needs "+r.Needs)}
			row.Hint = page.Cmd("station tools install " + r.Needs)
		}
	case settings.StateOffHere:
		row.Mark, row.Cells = page.OffHere, []page.Value{page.Text("off here")}
	case settings.StateNotForThisMachine:
		row.Mark, row.Cells = page.NotForThisMachine, []page.Value{page.Texts("not for this machine", r.Why)}
	case settings.StateReading:
		row.Mark, row.Detail = page.Awaiting, page.Pending()
	}
	return row
}

func order(s settings.State) int {
	switch s {
	case settings.StateChangedSince:
		return 0
	case settings.StateNotApplied:
		return 1
	case settings.StateApplied:
		return 2
	case settings.StateOffHere:
		return 3
	case settings.StateReading:
		return 4
	case settings.StateNotForThisMachine:
		return 5
	}
	return 6
}

func sorted(rows []settings.Row) []settings.Row {
	out := append([]settings.Row(nil), rows...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && order(out[j].State) < order(out[j-1].State); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func items(rows []settings.Row, now time.Time) []page.Item {
	out := make([]page.Item, 0, len(rows))
	for _, r := range sorted(rows) {
		row := Row(r, now)
		out = append(out, page.Item{Row: &row})
	}
	return out
}

// value: the rows that are not for this machine, folded on one line with
// their reasons, unless every group is unfolded.
func notHere(rows []settings.Row) page.Row {
	var why []string
	for _, r := range rows {
		if !contains(why, r.Why) {
			why = append(why, r.Why)
		}
	}
	return page.Row{ID: "not-for-this-machine", Label: strconv.Itoa(len(rows)) + " not for this machine", Mark: page.NotForThisMachine, Cells: []page.Value{page.Texts(why...)}}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// value: the Always-on Mac group folded: none of it applied on this machine.
func alwaysOnFolded(rows []settings.Row) page.Row {
	return page.Row{ID: "always-on-mac", Label: "Always-on Mac", Mark: page.Absent, Cells: []page.Value{page.Texts(strconv.Itoa(len(rows)), "for a Mac that hosts servers")}}
}

func touched(rows []settings.Row) bool {
	for _, r := range rows {
		if r.State == settings.StateApplied || r.State == settings.StateChangedSince || r.State == settings.StateOffHere {
			return true
		}
	}
	return false
}

func List(rows []settings.Row, now time.Time, all bool, here page.Context) page.Page {
	// invariant: the Set by station group, then the Always-on Mac group folded
	// on one line until one of it is applied here, then what is not for this
	// machine folded on one line; --all unfolds both.
	var set, always, away []settings.Row
	for _, r := range rows {
		switch {
		case r.State == settings.StateNotForThisMachine:
			away = append(away, r)
		case r.Group == settings.AlwaysOnMac:
			always = append(always, r)
		default:
			set = append(set, r)
		}
	}
	p := page.Page{ID: ID, Command: "settings", Context: here, Actions: actions()}
	p.Sections = append(p.Sections, page.Section{ID: "set", Title: "Set by station", Fold: page.Auto, Columns: columns(), Items: items(set, now), PipeCounts: true})
	if len(always) > 0 {
		s := page.Section{ID: "always-on-mac", Title: "Always-on Mac", Columns: columns(), PipeCounts: true}
		if all || touched(always) {
			s.Items = items(always, now)
		} else {
			s.Uncounted, s.PipeCounts = true, false
			folded := alwaysOnFolded(always)
			s.Items = []page.Item{{Row: &folded}}
		}
		p.Sections = append(p.Sections, s)
	}
	if len(away) > 0 {
		s := page.Section{ID: "not-for-this-machine", Title: "Not for this machine", Columns: columns(), Uncounted: true}
		if all {
			s.Items = items(away, now)
		} else {
			folded := notHere(away)
			s.Items = []page.Item{{Row: &folded}}
		}
		p.Sections = append(p.Sections, s)
	}
	p.Hints = []page.Hint{*page.Cmd("station settings apply"), *page.Cmd("station settings <setting>")}
	return page.Seal(p)
}

func actions() []page.Action {
	return []page.Action{
		{Key: KeyApply, Label: "apply", On: page.OnRow, Priority: 1, Mutates: true},
		{Key: KeyDiff, Label: "diff", On: page.OnRow, Priority: 2},
		{Key: KeyOff, Label: "off here", On: page.OnRow, Priority: 3, Mutates: true, Confirm: &page.Confirm{Yes: "off"}},
	}
}

func OffConfirm(names []string) page.Confirm {
	return page.Confirm{Question: "Turn " + strings.Join(names, ", ") + " off here? station stops applying and watching it on this machine", Yes: "off"}
}

func ApplyConfirm(names []string) page.Confirm {
	what := strings.Join(names, ", ")
	if what == "" {
		what = "every setting not applied or changed since"
	}
	return page.Confirm{Question: "Apply " + what + "? A setting changed since is overwritten: yours is lost", Yes: "apply"}
}

func Show(r settings.Row, now time.Time, here page.Context) page.Page {
	// value: one setting's detail: its group, state, where it lives and what
	// it does, and the machines it lives on.
	head := Row(r, now)
	group := page.Text("Set by station")
	if r.Group == settings.AlwaysOnMac {
		group = page.Texts("Set by station", "Always-on Mac")
	}
	facts := []page.Item{
		{Fact: &page.Fact{Key: "group", Value: group}},
		{Fact: &page.Fact{Key: "does", Value: page.Text(r.Item().Description)}},
		{Fact: &page.Fact{Key: "lives on", Value: page.Text(livesOn(r.Item().Places))}},
	}
	if r.Where != "" {
		facts = append(facts, page.Item{Fact: &page.Fact{Key: "where", Value: page.Text(r.Where)}})
	}
	if !r.Since.IsZero() {
		facts = append(facts, page.Item{Fact: &page.Fact{Key: "applied", Value: page.Text(ago(r.Since, now))}})
	}
	p := page.Page{ID: ID, Command: "settings " + r.Item().Name, Context: here, Sections: []page.Section{
		{ID: "setting", Title: r.Item().Name, Role: page.Banner, Columns: columns(), Items: []page.Item{{Row: &head}}},
		{ID: "detail", Title: "Detail", Columns: pages.OutcomeColumns(), Items: facts},
	}}
	return page.Seal(p)
}

func livesOn(places []catalog.MachineKind) string {
	words := make([]string, 0, len(places))
	for _, p := range places {
		switch p {
		case catalog.Host:
			words = append(words, "Macs")
		case catalog.Linux:
			words = append(words, "Linux VMs")
		case catalog.MacOS:
			words = append(words, "macOS VMs")
		}
	}
	return strings.Join(words, ", ")
}

func Values(r settings.Row, before, after []settings.KeyValue, here page.Context) page.Page {
	// value: a setting that writes no file, what station would set against
	// what is there: one row per value, before then after.
	was := map[string]string{}
	for _, kv := range before {
		was[kv.Key] = kv.Value
	}
	rows := make([]page.Item, 0, len(after))
	for _, kv := range after {
		now := was[kv.Key]
		if now == "" {
			now = "unset"
		}
		row := page.Row{ID: kv.Key, Label: kv.Key, Mark: page.Absent, Cells: []page.Value{page.Text(now), page.Text(kv.Value)}}
		if now == kv.Value {
			row.Mark = page.OK
		}
		rows = append(rows, page.Item{Row: &row})
	}
	cols := []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "before", Kind: page.KindText, Keep: 40}, {Key: "after", Kind: page.KindText, Keep: 40}}
	s := page.Section{ID: "values", Title: r.Item().Name, Columns: cols, Items: rows}
	if len(rows) == 0 {
		head := Row(r, time.Time{})
		s.Items = []page.Item{{Row: &head}}
	}
	return page.Seal(page.Page{ID: ID, Command: "settings diff " + r.Item().Name, Context: here, Sections: []page.Section{s}})
}

func Changes(verb string, changes []settings.Change, here page.Context) page.Page {
	// value: each setting a verb ran on, its outcome and its steps.
	var rows []page.Row
	var extra []page.Section
	for _, c := range changes {
		extra = append(extra, values(c.Item, "before", c.Before)...)
		extra = append(extra, values(c.Item, "after", c.After)...)
		head := pages.Done(c.Item, c.Result, "")
		if len(c.Steps) == 1 {
			s := c.Steps[0]
			head.Detail = page.Text(step(s))
			rows = append(rows, head)
			continue
		}
		rows = append(rows, head)
		for i, s := range c.Steps {
			r := pages.Done(c.Item+"/"+strconv.Itoa(i), s.State, s.Detail)
			r.Label = s.Label
			rows = append(rows, r)
		}
	}
	return pages.Outcome("settings "+verb, here, nil, rows, "settings to "+verb, extra...)
}

func values(item, when string, list []settings.KeyValue) []page.Section {
	if len(list) == 0 {
		return nil
	}
	items := make([]page.Item, 0, len(list))
	for _, kv := range list {
		items = append(items, page.Item{Fact: &page.Fact{Key: kv.Key, Value: page.Text(kv.Value)}})
	}
	return []page.Section{{ID: item + "-" + when, Title: item + " " + when, Columns: pages.OutcomeColumns(), Items: items}}
}

func step(s settings.ChangeStep) string {
	if s.Detail == "" {
		return s.Label
	}
	return s.Label + ": " + s.Detail
}

type SettingJSON struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	Group     string     `json:"group"`
	State     string     `json:"state"`
	AppliedAt *time.Time `json:"applied_at"`
	Where     string     `json:"where,omitempty"`
	Why       string     `json:"why,omitempty"`
	Admin     bool       `json:"admin"`
}

type ListJSON struct {
	Schema   string        `json:"schema"`
	Findings []string      `json:"findings"`
	Settings []SettingJSON `json:"settings"`
}

func Object(r settings.Row) SettingJSON {
	o := SettingJSON{Name: r.Item().Name, Label: r.Item().Description, Group: string(r.Group), State: string(r.State), Where: r.Where, Why: r.Why, Admin: r.Item().NeedsRoot}
	if !r.Since.IsZero() {
		at := r.Since.UTC()
		o.AppliedAt = &at
	}
	return o
}

func ListContract(rows []settings.Row) ListJSON {
	body := ListJSON{Schema: "station.settings.list/v1", Findings: []string{}, Settings: make([]SettingJSON, 0, len(rows))}
	for _, r := range rows {
		body.Settings = append(body.Settings, Object(r))
	}
	return body
}

type ShowJSON struct {
	Schema  string      `json:"schema"`
	Setting SettingJSON `json:"setting"`
}

func ShowContract(r settings.Row) ShowJSON {
	return ShowJSON{Schema: "station.settings.show/v1", Setting: Object(r)}
}
