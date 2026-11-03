package power

import (
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

const (
	KeyAwake = "a"
	KeySleep = "s"
	RowAwake = "awake"
)

func columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "value", Kind: page.KindText, Keep: 100},
		{Key: "detail", Kind: page.KindText, Keep: 60},
		{Key: "more", Kind: page.KindText, Keep: 20, Whole: true},
	}
}

func clock(t time.Time, zone *time.Location) string {
	if zone == nil {
		zone = time.UTC
	}
	return t.In(zone).Format("15:04")
}

func left(r power.Report) string { return pages.Term(r.Session.Until.Sub(r.Now)) + " left" }

func flags(s power.Session) []string {
	var out []string
	if s.Lid {
		out = append(out, "lid")
	}
	if s.Battery {
		out = append(out, "battery")
	}
	return out
}

func until(s power.Session, zone *time.Location) string {
	return "until " + clock(s.Until, zone) + " (" + pages.Term(s.Until.Sub(s.StartedAt)) + ")"
}

func ended(e *power.End, zone *time.Location) page.Value {
	if e == nil {
		return page.Text("the Mac sleeps as macOS decides")
	}
	return page.Texts("ended "+clock(e.At, zone), e.Detail)
}

func stopped(r power.Report) bool {
	return r.Daemon == power.DaemonStopped || r.Daemon == power.DaemonAbsent
}

func daemonHint(r power.Report) *page.Hint {
	if r.Daemon == power.DaemonAbsent {
		return page.Cmd("station setup")
	}
	return page.Cmd("station start")
}

func sessionRows(r power.Report) []page.Row {
	switch {
	case r.Daemon == power.DaemonPending:
		return []page.Row{{ID: RowAwake, Label: RowAwake, Mark: page.Awaiting, Cells: []page.Value{page.Pending()}}}
	case stopped(r):
		return []page.Row{{ID: RowAwake, Label: RowAwake, Mark: page.Attention, Reason: page.Stopped, Cells: []page.Value{page.Unknown(), page.Text("daemon " + r.Daemon)}, Hint: daemonHint(r)}}
	case r.Session == nil:
		return []page.Row{{ID: RowAwake, Label: RowAwake, Mark: page.OK, Cells: []page.Value{page.Off(), ended(r.LastEnd, r.Zone)}, Hint: page.KeyHint("station power awake", KeyAwake)}}
	}
	s := *r.Session
	rows := []page.Row{{ID: RowAwake, Label: RowAwake, Mark: page.OK, Cells: []page.Value{
		page.On(), page.Texts(append(flags(s), until(s, r.Zone))...), page.Text("started " + clock(s.StartedAt, r.Zone) + " by " + s.By),
	}}}
	if s.Lid {
		rows = append(rows,
			page.Row{ID: "display", Label: "display", Mark: page.OK, Cells: []page.Value{page.Text("sleeps"), page.Text("on lid close: pmset displaysleepnow (no display assertion)")}},
			page.Row{ID: "clamshell", Label: "clamshell", Mark: page.OK, Cells: []page.Value{page.Text("held"), page.Text("reset to normal at the end, at every daemon start and quit")}},
		)
	} else {
		rows = append(rows, page.Row{ID: "display", Label: "display", Mark: page.OK, Cells: []page.Value{page.Text("as set"), page.Text("the display sleeps on its own timer (no display assertion)")}})
	}
	return rows
}

func near(r power.Report, name string) bool {
	for _, g := range r.Guards {
		if g.Name == name {
			return g.State == power.GuardNear
		}
	}
	return false
}

func source(rd power.Reading) string {
	if rd.Source == "battery" {
		return "on battery"
	}
	return "AC attached"
}

func label(name string) string {
	if name == power.ReasonLowPower {
		return "low power"
	}
	return name
}

type guard struct {
	name         string
	value        page.Value
	far, closeTo string
}

func guards(r power.Report) []guard {
	rd := r.Reading
	battery := page.Text("no battery")
	if rd.HasBattery {
		battery = page.Texts(strconv.Itoa(rd.Percent)+"%", source(rd))
	}
	lid := page.Text(rd.Lid)
	switch {
	case rd.Lid == "":
		lid = page.Text("no lid")
	case rd.Lid == "closed" && rd.Source == "battery" && rd.HasBattery:
		lid = page.Text("closed on battery")
	}
	low := page.Off()
	if rd.LowPower {
		low = page.On()
	}
	return []guard{
		{power.ReasonBattery, battery, "ends a session at 20% on battery", "the session ends at 20%"},
		{power.ReasonLowPower, low, "ends a session when it turns on", ""},
		{power.ReasonThermal, page.Text(power.Level(rd.Thermal)), "ends a session at serious, or heavy for 5m lid closed on battery", "the session ends at serious"},
		{power.ReasonLid, lid, "closing it on battery ends a --lid session without --battery", "allowed for this session"},
	}
}

func guardRow(r power.Report, g guard, records bool) page.Row {
	row := page.Row{ID: g.name, Label: label(g.name), Mark: page.OK, Cells: []page.Value{g.value}}
	if near(r, g.name) {
		row.Mark, row.Reason = page.Attention, page.Guard
		row.Cells = append(row.Cells, page.Text(g.closeTo))
		return row
	}
	if !records {
		row.Cells = append(row.Cells, page.Text(g.far))
	}
	return row
}

func reading(rd power.Reading) []page.Item {
	lid := rd.Lid
	if lid == "" {
		lid = "none"
	}
	src := page.Text("AC")
	if rd.Source == "battery" {
		src = page.Text("battery")
	}
	if rd.HasBattery {
		charging := "not charging"
		if rd.Charging {
			charging = "charging"
		}
		src = page.Texts(src.Text, strconv.Itoa(rd.Percent)+"%", charging)
	}
	low := page.Off()
	if rd.LowPower {
		low = page.On()
	}
	facts := []page.Fact{
		{Key: "lid", Value: page.Text(lid)},
		{Key: "source", Value: src},
		{Key: "low power", Value: low},
		{Key: "thermal", Value: page.Text(power.Level(rd.Thermal) + " (" + strconv.Itoa(rd.Thermal) + ")")},
	}
	items := make([]page.Item, 0, len(facts))
	for i := range facts {
		items = append(items, page.Item{Fact: &facts[i]})
	}
	return items
}

func items(rows []page.Row) []page.Item {
	out := make([]page.Item, 0, len(rows))
	for i := range rows {
		out = append(out, page.Item{Row: &rows[i]})
	}
	return out
}

func session(r power.Report) page.Section {
	s := page.Section{ID: "power", Title: "Power", Columns: columns(), Items: items(sessionRows(r))}
	if r.Session != nil && r.Daemon == power.DaemonRunning {
		s.Role = page.Banner
		s.Caption = []page.Value{page.Texts(append(append([]string{"awake"}, flags(*r.Session)...), left(r))...)}
	}
	return s
}

func Page(r power.Report, here page.Context) page.Page {
	gs := guards(r)
	rows := make([]page.Row, 0, len(gs))
	for _, g := range gs {
		rows = append(rows, guardRow(r, g, false))
	}
	p := page.Page{ID: "power", Command: "power", Context: here, Sections: []page.Section{
		session(r),
		{ID: "guards", Title: "Guards", Columns: columns(), Items: items(rows)},
		{ID: "reading", Title: "Reading", Uncounted: true, Columns: columns(), Items: reading(r.Reading)},
	}}
	if r.Daemon == power.DaemonPending {
		p.State = page.Loading
	}
	p.Actions = Actions()
	return page.Seal(p)
}

func Actions() []page.Action {
	return []page.Action{
		{Key: KeyAwake, Label: "awake", On: page.OnPage, Priority: 1, Mutates: true},
		{Key: KeySleep, Label: "sleep", On: page.OnPage, Priority: 2, Mutates: true},
	}
}

func Records(r power.Report) page.Page {
	var rows []page.Row
	switch {
	case r.Session != nil && r.Daemon == power.DaemonRunning:
		s := *r.Session
		cells := make([]page.Value, 0, 5)
		cells = append(cells, page.On())
		for _, f := range flags(s) {
			cells = append(cells, page.Text(f))
		}
		cells = append(cells, page.Text(s.Until.UTC().Format(time.RFC3339)), page.Text(s.By))
		rows = append(rows, page.Row{ID: RowAwake, Label: RowAwake, Mark: page.OK, Cells: cells})
		for _, row := range sessionRows(r)[1:] {
			row.Cells = row.Cells[:1]
			rows = append(rows, row)
		}
	case r.Session == nil && r.Daemon == power.DaemonRunning:
		rows = append(rows, page.Row{ID: RowAwake, Label: RowAwake, Mark: page.OK, Cells: []page.Value{page.Off()}})
	default:
		row := sessionRows(r)[0]
		if row.Hint != nil {
			row.Cells = append(row.Cells, page.Text(row.Hint.Command))
			row.Hint = nil
		}
		rows = append(rows, row)
	}
	for _, g := range guards(r) {
		rows = append(rows, guardRow(r, g, true))
	}
	return page.Seal(page.Page{ID: "power", Command: "power", Sections: []page.Section{
		{ID: "records", Title: "Power", Role: page.Banner, Columns: columns(), Items: items(rows)},
		{ID: "reading", Title: "Reading", Uncounted: true, Columns: columns(), Items: reading(r.Reading)},
	}})
}

type ReadingJSON struct {
	Lid          string `json:"lid,omitempty"`
	Source       string `json:"source"`
	Percent      *int   `json:"battery_percent,omitempty"`
	Charging     bool   `json:"charging"`
	LowPower     bool   `json:"low_power"`
	Thermal      string `json:"thermal"`
	ThermalLevel int    `json:"thermal_level"`
}

type DaemonJSON struct {
	Running bool   `json:"running"`
	State   string `json:"state,omitempty"`
}

type JSON struct {
	Schema  string         `json:"schema"`
	Session *power.Session `json:"session"`
	LastEnd *power.End     `json:"last_end"`
	Reading ReadingJSON    `json:"reading"`
	Guards  []power.Guard  `json:"guards"`
	Daemon  DaemonJSON     `json:"daemon"`
}

func Contract(r power.Report) JSON {
	rd := r.Reading
	out := JSON{
		Schema: "station.power/v1", Session: r.Session, LastEnd: r.LastEnd, Guards: r.Guards,
		Reading: ReadingJSON{Lid: rd.Lid, Source: rd.Source, Charging: rd.Charging, LowPower: rd.LowPower, Thermal: power.Level(rd.Thermal), ThermalLevel: rd.Thermal},
		Daemon:  DaemonJSON{Running: r.Daemon == power.DaemonRunning},
	}
	if rd.HasBattery {
		n := rd.Percent
		out.Reading.Percent = &n
	}
	if !out.Daemon.Running {
		out.Daemon.State = r.Daemon
	}
	return out
}

func AwakeForm(host string) page.Form {
	terms := []string{"1h", "2h", "4h", "8h", "12h", "24h"}
	opts := make([]page.Option, 0, len(terms))
	for _, t := range terms {
		opts = append(opts, page.Option{Value: t})
	}
	return page.Form{Title: "Keep " + host + " awake", Fields: []page.Field{
		{Key: "for", Label: "For", Kind: page.FieldSelect, Options: opts, Default: "4h"},
		{Key: "lid", Label: "Lid", Kind: page.FieldConfirm, Note: "keep awake with the lid closed (the display sleeps)"},
		{Key: "battery", Label: "Battery", Kind: page.FieldConfirm, Note: "allow the lid closed on battery for this session"},
	}}
}

func Awake(a power.Awoken, zone *time.Location, here page.Context) page.Page {
	r := pages.Done(RowAwake, catalog.StateOK, "")
	words := []string{"on"}
	if a.Refreshed {
		words = []string{"refreshed"}
	} else {
		words = append(words, flags(a.Session)...)
	}
	r.Detail = page.Texts(append(words, until(a.Session, zone))...)
	return pages.Outcome("power awake", here, nil, []page.Row{r}, "Power session")
}

func Slept(s power.Slept, here page.Context) page.Page {
	if !s.Ended {
		return pages.Outcome("power sleep", here, nil, nil, "Power session to end")
	}
	r := pages.Done(RowAwake, catalog.StateOK, "")
	r.Detail = page.Texts("off", "ended after "+pages.Term(s.After))
	return pages.Outcome("power sleep", here, nil, []page.Row{r}, "Power session to end")
}
