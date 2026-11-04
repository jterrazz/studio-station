package apps

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

func Outcomes(verb string, outcomes []apps.Outcome, here page.Context) page.Page {
	rows := make([]page.Row, 0, len(outcomes))
	for _, o := range outcomes {
		rows = append(rows, pages.Done(o.Name, o.Result, o.Detail))
		for _, s := range o.Steps {
			step := pages.Done(o.Name+"/"+s.Item, s.Result, s.Detail)
			step.Label = s.Item
			rows = append(rows, step)
		}
	}
	return pages.Outcome("apps "+verb, here, nil, rows, "apps to "+verb)
}

func Updated(outcomes []apps.Outcome, here page.Context) page.Page {
	// invariant: an update with nothing to update says every app is up to
	// date, a calm line, never an empty list.
	if len(outcomes) > 0 {
		return Outcomes("update", outcomes, here)
	}
	label := here.Machine
	if here.Kind == "host" || label == "" {
		label = "This Mac"
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
