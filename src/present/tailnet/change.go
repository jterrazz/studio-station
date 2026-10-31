package tailnet

import (
	"strings"

	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/present"
)

func Up(r tailnet.UpResult, context string) present.Answer {
	rows := []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{
		{Text: "tailnet", Tone: present.Strong},
		{Text: "connected (" + string(r.Daemon) + " daemon)"},
	}}}
	if r.Daemon == tailnet.Userspace && !r.KeepAwake {
		rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
			{Text: "keep awake", Tone: present.Strong},
			{Text: "not active"},
		}})
	}
	return answer("up", context, rows)
}

func Down(r tailnet.DownResult, context string) present.Answer {
	text := "already disconnected"
	names := make([]string, len(r.Stopped))
	for i, d := range r.Stopped {
		names[i] = string(d)
	}
	if len(names) > 0 {
		text = "disconnected (" + strings.Join(names, " + ") + " daemon)"
	}
	rows := []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{{Text: "tailnet", Tone: present.Strong}, {Text: text}}}}
	return answer("down", context, rows)
}

func answer(verb, context string, rows []present.Row) present.Answer {
	return present.Answer{Doc: present.Document{
		Command: "station tailnet " + verb,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
