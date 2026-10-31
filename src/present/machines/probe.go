package machines

import (
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/present"
)

type checkJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type probeJSON struct {
	Schema  string      `json:"schema"`
	Name    string      `json:"name"`
	User    string      `json:"user"`
	IP      string      `json:"ip"`
	Checks  []checkJSON `json:"checks"`
	Console string      `json:"console"`
}

func Probe(r machines.ProbeReport, context string) present.Answer {
	body := probeJSON{Schema: "station.machines.probe/v1", Name: r.Name, User: r.User, IP: r.IP, Console: r.Console, Checks: []checkJSON{}}
	rows := make([]present.Row, 0, len(r.Checks))
	for _, c := range r.Checks {
		body.Checks = append(body.Checks, checkJSON{Name: c.Name, State: string(c.State), Value: c.Value, Detail: c.Detail})
		rows = append(rows, present.Row{Mark: present.MarkOf(c.State), Cells: []present.Cell{
			{Text: c.Name, Tone: present.Strong},
			{Text: c.Value},
			{Text: c.Detail, Tone: present.Muted},
		}})
	}
	return present.Answer{
		Doc:  present.Document{Command: "station machines probe " + r.Name, Context: context, Blocks: []present.Block{present.Table{Rows: rows}}},
		JSON: body,
	}
}
