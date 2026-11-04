package awake

import (
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/awake"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const Schema = "station.awake.read/v1"

type Silent struct {
	// value: a Mac Awake could not read: its name and why.
	Mac, Why string
}

type Fleet struct {
	Now     time.Time
	Zone    *time.Location
	Reports []awake.Report
	Silent  []Silent
}

func columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "mode", Kind: page.KindText, Keep: 100},
	}
}

func clock(t, now time.Time, zone *time.Location) string {
	if zone == nil {
		zone = time.UTC
	}
	at, today := t.In(zone), now.In(zone)
	words := at.Format("15:04")
	if at.YearDay() != today.YearDay() || at.Year() != today.Year() {
		words += " tomorrow (" + at.Format("Mon 2 Jan") + ")"
	}
	return words
}

func stopped(r awake.Report) bool {
	return r.Daemon != "" && r.Daemon != awake.EngineRunning && r.Daemon != awake.EngineReading
}

func until(s awake.Session, now time.Time, zone *time.Location) []string {
	words := []string{"Awake until " + clock(s.Until, now, zone)}
	if s.Lid {
		words = append(words, "lid closed")
	}
	if s.Battery {
		words = append(words, "on battery allowed")
	}
	return append(words, pages.Term(s.Until.Sub(now))+" left")
}

func hosts(servers []string) string {
	parts := make([]string, 0, len(servers))
	for _, s := range servers {
		parts = append(parts, s+" (server)")
	}
	return "hosts " + strings.Join(parts, ", ")
}

func never(r awake.Report) []string {
	words := []string{"Never sleeps", "always-on Mac policy"}
	if !r.NeverHeld {
		words = append(words, "let go while the battery is low")
	} else if !r.NeverSince.IsZero() {
		words = append(words, "last slept: never in "+pages.Term(r.Now.Sub(r.NeverSince)))
	}
	if len(r.Servers) > 0 {
		words = append(words, hosts(r.Servers))
	}
	return words
}

func Row(r awake.Report) page.Row {
	row := page.Row{ID: r.Mac, Label: r.Mac}
	switch {
	case r.Daemon == awake.EngineReading:
		row.Mark, row.Cells = page.Awaiting, []page.Value{page.Pending()}
	case stopped(r):
		row.Mark, row.Cells = page.Absent, []page.Value{page.Texts("Sleeps normally", "the engine is stopped")}
		row.Hint = page.Cmd("station engine start")
	default:
		switch r.Mode() {
		case awake.NeverSleeps:
			row.Mark, row.Cells = page.OK, []page.Value{page.Texts(never(r)...)}
			if !r.NeverHeld {
				row.Mark, row.Reason = page.Attention, page.Guard
			}
		case awake.AwakeUntil:
			row.Mark, row.Cells = page.OK, []page.Value{page.Texts(until(*r.Session, r.Now, r.Zone)...)}
		case awake.StoppedByGuard:
			row.Mark, row.Reason = page.Attention, page.Guard
			row.Cells = []page.Value{page.Texts("Awake stopped", r.LastEnd.Detail)}
		case awake.SleepsNormally:
			row.Mark, row.Cells = page.Absent, []page.Value{page.Text("Sleeps normally")}
		}
	}
	return row
}

func source(rd awake.Reading) page.Value {
	if !rd.HasBattery {
		return page.Text("AC")
	}
	src := "AC"
	if rd.Source == "battery" {
		src = "battery"
	}
	charging := "not charging"
	if rd.Charging {
		charging = "charging"
	}
	return page.Texts(src, strconv.Itoa(rd.Percent)+" %", charging)
}

func reading(r awake.Report) page.Section {
	rd := r.Reading
	var facts []page.Fact
	if r.Caps.Lid {
		facts = append(facts, page.Fact{Key: "lid", Value: page.Text(rd.Lid)})
	}
	facts = append(facts, page.Fact{Key: "power", Value: source(rd)})
	if r.Caps.Thermal {
		facts = append(facts, page.Fact{Key: "thermal", Value: page.Text(awake.Level(rd.Thermal))})
	}
	facts = append(facts, page.Fact{Key: "screen", Value: page.Text("always turns off")})
	items := make([]page.Item, 0, len(facts))
	for i := range facts {
		items = append(items, page.Item{Fact: &facts[i]})
	}
	return page.Section{ID: "reading", Title: r.Mac, Uncounted: true, Columns: columns(), Items: items}
}

func Page(f Fleet, here page.Context) page.Page {
	// invariant: every Mac, This Mac first, each by its mode; its readings
	// are the ones it reports, This Mac's only.
	rows := make([]page.Row, 0, len(f.Reports)+len(f.Silent))
	for _, r := range f.Reports {
		rows = append(rows, Row(r))
	}
	for _, s := range f.Silent {
		rows = append(rows, page.Row{ID: s.Mac, Label: s.Mac, Mark: page.NotKnown, Reason: page.Unreachable, Cells: []page.Value{page.Text(s.Why)}})
	}
	items := make([]page.Item, 0, len(rows))
	for i := range rows {
		items = append(items, page.Item{Row: &rows[i]})
	}
	sections := []page.Section{{ID: "awake", Title: "Awake", Columns: columns(), Items: items}}
	if len(f.Reports) > 0 && f.Reports[0].Here && !stopped(f.Reports[0]) && f.Reports[0].Daemon != awake.EngineReading {
		sections = append(sections, reading(f.Reports[0]))
	}
	p := page.Page{ID: "awake", Command: "awake", Context: here, Sections: sections}
	for _, r := range f.Reports {
		if r.Daemon == awake.EngineReading {
			p.State = page.Loading
		}
	}
	return page.Seal(p)
}

type CapsJSON struct {
	Battery bool `json:"battery"`
	Lid     bool `json:"lid"`
	Thermal bool `json:"thermal"`
}

type ReadingJSON struct {
	Lid      string `json:"lid,omitempty"`
	Source   string `json:"source"`
	Percent  *int   `json:"battery_percent,omitempty"`
	Charging bool   `json:"charging"`
	LowPower bool   `json:"low_power"`
	Thermal  string `json:"thermal,omitempty"`
}

type MacJSON struct {
	Name         string       `json:"name"`
	Here         bool         `json:"here"`
	Mode         awake.Mode   `json:"mode"`
	Engine       string       `json:"engine"`
	Until        *time.Time   `json:"until,omitempty"`
	Lid          bool         `json:"lid,omitempty"`
	Battery      bool         `json:"battery,omitempty"`
	StoppedBy    string       `json:"stopped_by,omitempty"`
	NeverSince   *time.Time   `json:"never_since,omitempty"`
	Servers      []string     `json:"servers"`
	Capabilities CapsJSON     `json:"capabilities"`
	Reading      *ReadingJSON `json:"reading,omitempty"`
}

type SilentJSON struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type JSON struct {
	Schema string       `json:"schema"`
	Macs   []MacJSON    `json:"macs"`
	Silent []SilentJSON `json:"silent"`
}

func macJSON(r awake.Report) MacJSON {
	m := MacJSON{Name: r.Mac, Here: r.Here, Mode: r.Mode(), Engine: r.Daemon, Servers: r.Servers,
		Capabilities: CapsJSON{Battery: r.Caps.Battery, Lid: r.Caps.Lid, Thermal: r.Caps.Thermal}}
	if m.Servers == nil {
		m.Servers = []string{}
	}
	if stopped(r) {
		m.Mode = awake.SleepsNormally
	}
	if s := r.Session; s != nil && m.Mode == awake.AwakeUntil {
		u := s.Until.UTC()
		m.Until, m.Lid, m.Battery = &u, s.Lid, s.Battery
	}
	if m.Mode == awake.StoppedByGuard {
		m.StoppedBy = r.LastEnd.Reason
	}
	if m.Mode == awake.NeverSleeps && !r.NeverSince.IsZero() {
		since := r.NeverSince.UTC()
		m.NeverSince = &since
	}
	if r.Here {
		rd := r.Reading
		j := &ReadingJSON{Source: rd.Source, Charging: rd.Charging, LowPower: rd.LowPower}
		if r.Caps.Lid {
			j.Lid = rd.Lid
		}
		if r.Caps.Battery {
			n := rd.Percent
			j.Percent = &n
		}
		if r.Caps.Thermal {
			j.Thermal = awake.Level(rd.Thermal)
		}
		m.Reading = j
	}
	return m
}

func Contract(f Fleet) JSON {
	out := JSON{Schema: Schema, Macs: []MacJSON{}, Silent: []SilentJSON{}}
	for _, r := range f.Reports {
		out.Macs = append(out.Macs, macJSON(r))
	}
	for _, s := range f.Silent {
		out.Silent = append(out.Silent, SilentJSON{Name: s.Mac, Error: s.Why})
	}
	return out
}

func Kept(a awake.Awoken, label string, now time.Time, zone *time.Location, here page.Context) page.Page {
	if a.NeverSleeps {
		r := pages.Done(a.Mac, catalog.StateOK, "already never sleeps (always-on Mac policy). Nothing to add.")
		r.Hint = page.Cmd("station settings off never-sleeps" + on(a.Mac, label, here))
		return pages.Outcome("awake", here, nil, []page.Row{r}, "keep-awake")
	}
	r := pages.Done(a.Mac, catalog.StateOK, "")
	words := []string{"stays awake until " + clock(a.Until, now, zone)}
	if a.Refreshed {
		words[0] = "now " + words[0]
	}
	if a.Lid {
		words = append(words, "lid closed")
	}
	if a.Battery {
		words = append(words, "on battery allowed")
	}
	r.Detail = page.Texts(words...)
	return pages.Outcome("awake", here, nil, []page.Row{r}, "keep-awake")
}

func on(mac, label string, here page.Context) string {
	if mac == "" || mac == here.Machine {
		return ""
	}
	return " --on " + label
}

func Off(s awake.Slept, label string, here page.Context) page.Page {
	if s.Ended {
		r := pages.Done(s.Mac, catalog.StateOK, "sleeps normally again")
		return pages.Outcome("awake off", here, nil, []page.Row{r}, "keep-awake to end")
	}
	r := pages.Done(s.Mac, catalog.StateAbsent, "has no keep-awake to end")
	if s.NeverSleeps {
		r.Detail = page.Texts("has no keep-awake to end", "Never sleeps stays: station settings off never-sleeps"+on(s.Mac, label, here))
	}
	return pages.Outcome("awake off", here, nil, []page.Row{r}, "keep-awake to end")
}
