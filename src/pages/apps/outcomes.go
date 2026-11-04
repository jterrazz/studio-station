package apps

import (
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

type Run struct {
	// value: the machine the verb ran on ("" for this one) and how long it took.
	On   string
	Took time.Duration
}

func Outcomes(verb string, outcomes []apps.Outcome, run Run, here page.Context) page.Page {
	// invariant: an outcome opens with what changed, counted, on which
	// machine and in how long (`Installed 2 apps on This Mac · 9s`), then
	// each app with the version and the method that now hold it.
	rows := make([]page.Row, 0, len(outcomes))
	changed, failed := 0, 0
	for _, o := range outcomes {
		row := pages.Done(o.Name, o.Result, o.Detail)
		row.Cells = []page.Value{page.Version(o.Version), page.Texts(string(o.Method))}
		rows = append(rows, row)
		switch {
		case o.Result == catalog.StateBroken:
			failed++
		case o.Result == catalog.StateOK && o.Detail != apps.AlreadyInstalled:
			changed++
		}
		for _, s := range o.Steps {
			step := pages.Done(o.Name+"/"+s.Item, s.Result, s.Detail)
			step.Label = s.Item
			rows = append(rows, step)
		}
	}
	p := pages.Outcome("apps "+verb, here, Columns(), rows, "apps to "+verb)
	if changed == 0 {
		return p
	}
	head := page.Row{ID: "summary/" + verb, Label: done(verb) + " " + plural(changed, "app") + where(verb, run.On, here), Mark: page.OK}
	var words []string
	if failed > 0 {
		head.Mark = page.Broken
		words = append(words, strconv.Itoa(failed)+" failed")
	}
	if took := run.Took.Round(time.Second); took > 0 {
		words = append(words, pages.Term(took))
	}
	head.Cells = []page.Value{page.Texts(words...)}
	summary := page.Section{ID: "summary", Title: "Summary", Role: page.Summary, Fold: page.Open, Uncounted: true, Columns: []page.Column{{Key: "area", Kind: page.KindText, Keep: 100}, {Key: "value", Kind: page.KindList, Keep: 60}}, Items: []page.Item{{Row: &head}}}
	p.Sections = append([]page.Section{summary}, p.Sections...)
	return page.Seal(p)
}

func done(verb string) string {
	switch verb {
	case "install":
		return "Installed"
	case "update":
		return "Updated"
	case "remove":
		return "Removed"
	}
	return verb
}

func where(verb, on string, here page.Context) string {
	label := on
	if label == "" {
		label = machineLabel(here)
	}
	if verb == "remove" {
		return " from " + label
	}
	return " on " + label
}

func machineLabel(here page.Context) string {
	if here.Kind == string(catalog.Host) || here.Machine == "" {
		return "This Mac"
	}
	return here.Machine
}

func Updated(outcomes []apps.Outcome, run Run, here page.Context) page.Page {
	// invariant: an update with nothing to update says every app is up to
	// date, a calm line, never an empty list.
	if len(outcomes) > 0 {
		return Outcomes("update", outcomes, run, here)
	}
	label := machineLabel(here)
	if run.On != "" {
		label = run.On
	}
	row := page.Row{ID: "up-to-date", Label: "every app is up to date on " + label, Mark: page.OK}
	return pages.Outcome("apps update", here, nil, []page.Row{row}, "apps to update")
}

type MachineUpdate struct {
	Machine string
	// value: what the machine answered; Skipped and Err say why it did not.
	Outcomes []apps.Outcome
	Skipped  string
	Err      string
}

func EveryMachine(updates []MachineUpdate, here page.Context) page.Page {
	// invariant: --all-machines tells one line per machine: what it updated,
	// why it was skipped (a server, a stopped VM), or that it did not answer.
	rows := make([]page.Row, 0, len(updates))
	for _, u := range updates {
		row := page.Row{ID: u.Machine, Label: u.Machine, Mark: page.OK}
		switch {
		case u.Err != "":
			row.Mark, row.Reason, row.Detail = page.Broken, page.Failing, page.Text(u.Err)
		case u.Skipped != "":
			row.Mark, row.Detail = page.NotForThisMachine, page.Text("skipped ("+u.Skipped+")")
		default:
			failed, _ := apps.FailedOf(u.Outcomes)
			words := []string{"every app is up to date"}
			if n := len(u.Outcomes) - failed; n > 0 {
				words[0] = "updated " + plural(n, "app")
			}
			if failed > 0 {
				row.Mark, row.Reason = page.Broken, page.Failing
				words = append(words, strconv.Itoa(failed)+" failed")
			}
			row.Detail = page.Texts(words...)
		}
		rows = append(rows, row)
	}
	return pages.Outcome("apps update", here, nil, rows, "machines to update")
}
