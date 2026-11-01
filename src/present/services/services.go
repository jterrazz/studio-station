package services

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/present"
)

func Fleet(f services.Fleet, context string) present.Answer {
	// invariant: one row schema everywhere, tty and pipe: mark, vm, guest port,
	// public URL, host, state, process, detail; the host on every row, so
	// `awk -F'\t' '$5=="macbook"'` keeps one host's services.
	var rows []present.Row
	for _, h := range f.Hosts {
		switch {
		case !h.Reachable:
			rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
				{Text: h.Name, Tone: present.Strong}, {}, {}, {Text: h.Name, Tone: present.Muted},
				{Text: "unreachable", Tone: present.Warn}, {}, {Text: "ssh " + h.Name + " true", Tone: present.Muted},
			}})
			continue
		case h.Here && !h.Daemon.Running:
			rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
				{Text: "daemon", Tone: present.Strong}, {}, {}, {Text: h.Name, Tone: present.Muted},
				{Text: "not running", Tone: present.Warn}, {}, {Text: "station setup", Tone: present.Muted},
			}})
		}
		for _, s := range h.Services {
			rows = append(rows, Row(s, h.Name))
		}
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Mark: present.MarkAbsent, Cells: []present.Cell{{Text: "no service yet", Tone: present.Muted}}})
	}
	return present.Answer{
		Doc:  present.Document{Command: "station services", Context: context, Blocks: []present.Block{present.Table{Rows: rows}}},
		JSON: f,
	}
}

func markOf(s services.View) present.Mark {
	switch {
	case s.State == services.Refused:
		return present.MarkBroken
	case s.State == services.Unknown || s.Degraded:
		return present.MarkDegraded
	case s.State == services.Waiting:
		return present.MarkAbsent
	}
	return present.MarkOK
}

func Row(s services.View, host string) present.Row {
	public := s.URLs.Public()
	if s.HostPort == 0 {
		public = ""
	}
	detail := s.Detail
	if detail == "" {
		detail = s.Profile
	}
	return present.Row{Mark: markOf(s), Cells: []present.Cell{
		{Text: s.VM, Tone: present.Strong},
		{Text: strconv.Itoa(s.GuestPort)},
		{Text: public},
		{Text: host, Tone: present.Muted},
		{Text: string(s.State)},
		{Text: s.Process, Tone: present.Muted},
		{Text: detail, Tone: present.Muted},
	}}
}

func URL(s services.View, context string) present.Answer {
	public := s.URLs.Public()
	return present.Answer{
		Doc: present.Document{
			Command: "station services url " + strconv.Itoa(s.GuestPort),
			Context: context,
			Blocks:  []present.Block{present.Table{Rows: []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{{Text: strconv.Itoa(s.GuestPort) + " → " + public, Tone: present.Strong}}}}}},
		},
		JSON:  s,
		Piped: public + "\n",
	}
}

func Released(l services.Lease, context string) present.Answer {
	return present.Answer{Doc: present.Document{
		Command: "station services release",
		Context: context,
		Blocks: []present.Block{present.Table{Rows: []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: l.VM + ":" + strconv.Itoa(l.GuestPort), Tone: present.Strong},
			{Text: "released", Tone: present.Muted},
		}}}}},
	}}
}
