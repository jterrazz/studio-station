package services

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/present"
)

func Fleet(f services.Fleet, context string) present.Answer {
	// invariant: one row schema everywhere, tty and pipe: mark, vm, guest port,
	// public URL, host, state, process, detail; the host on every row, so
	// `awk -F'\t' '$5=="macbook"'` keeps one host's services. Each host's range
	// heads its group on a terminal only: the pipe stays rows.
	var blocks []present.Block
	total := 0
	for _, h := range f.Hosts {
		var rows []present.Row
		switch {
		case !h.Reachable:
			rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
				{Text: h.Name, Tone: present.Strong}, {}, {}, {Text: h.Name, Tone: present.Muted},
				{Text: "unreachable", Tone: present.Warn}, {}, {Text: "ssh " + h.Name + " true", Tone: present.Muted},
			}})
		case h.Here && !h.Daemon.Running:
			rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
				{Text: "daemon", Tone: present.Strong}, {}, {}, {Text: h.Name, Tone: present.Muted},
				{Text: "not running", Tone: present.Warn}, {}, {Text: "station setup", Tone: present.Muted},
			}})
		}
		if h.Reachable {
			for _, s := range h.Services {
				rows = append(rows, Row(s, h.Name))
			}
		}
		total += len(rows)
		blocks = append(blocks, present.Section{Title: Heading(h), Banner: true, Blocks: []present.Block{present.Table{Rows: rows}}})
	}
	if total == 0 {
		blocks = append(blocks, present.Table{Rows: []present.Row{{Mark: present.MarkAbsent, Cells: []present.Cell{{Text: "no service yet", Tone: present.Muted}}}}})
	}
	return present.Answer{
		Doc:  present.Document{Command: "station services", Context: context, Blocks: blocks},
		JSON: f,
	}
}

func Heading(h services.HostView) string {
	if h.Range == nil {
		return h.Name
	}
	r := h.Range
	return h.Name + "  " + strconv.Itoa(r.Low) + "-" + strconv.Itoa(r.High) +
		" · " + strconv.Itoa(r.Leased) + " leased · " + strconv.Itoa(r.Free) + " free · " +
		strconv.Itoa(r.MaxPerVM) + " per VM"
}

func markOf(s services.View) present.Mark {
	switch {
	case s.State == services.Refused:
		return present.MarkBroken
	case s.State == services.Unknown:
		return present.MarkDegraded
	case s.State == services.Waiting:
		return present.MarkAbsent
	case s.Detail != "":
		return present.MarkDegraded
	}
	return present.MarkOK
}

func Row(s services.View, host string) present.Row {
	public := s.URLs.Public
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
	public := s.URLs.Public
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
