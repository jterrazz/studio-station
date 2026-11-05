package home

import "github.com/jterrazz/studio-station/src/overview"

type SummaryFixJSON struct {
	// invariant: one set of the three: a call station runs, a command the
	// owner runs, or a path in System Settings.
	Call    string            `json:"call,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Command string            `json:"command,omitempty"`
	Path    string            `json:"path,omitempty"`
}

type SummaryFindingJSON struct {
	Machine  string         `json:"machine"`
	Section  string         `json:"section"`
	Severity string         `json:"severity"`
	Subject  string         `json:"subject"`
	Text     string         `json:"text"`
	Fix      SummaryFixJSON `json:"fix"`
}

type SummarySectionJSON struct {
	ID string `json:"id"`
	// value: the line's counts by their word ("ok", "not installed").
	Counts  map[string]int `json:"counts"`
	Pending bool           `json:"pending,omitempty"`
	Unread  string         `json:"unread,omitempty"`
}

type SummaryMachineJSON struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type SummaryVMJSON struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type SummaryJSON struct {
	Schema   string               `json:"schema"`
	Machine  SummaryMachineJSON   `json:"machine"`
	Engine   string               `json:"engine,omitempty"`
	Findings []SummaryFindingJSON `json:"findings"`
	Sections []SummarySectionJSON `json:"sections"`
	VMs      []SummaryVMJSON      `json:"vms,omitempty"`
}

func SummaryContract(s overview.Summary, schema string) SummaryJSON {
	out := SummaryJSON{Schema: schema, Machine: SummaryMachineJSON{Name: s.Head.Name, Kind: string(s.Head.Kind)}, Engine: string(s.Engine),
		Findings: make([]SummaryFindingJSON, 0, len(s.Needs)), Sections: make([]SummarySectionJSON, 0, len(s.Lines))}
	for _, n := range s.Needs {
		out.Findings = append(out.Findings, SummaryFindingJSON{Machine: n.Machine, Section: string(n.Section), Severity: string(n.Severity), Subject: n.Subject, Text: n.Text,
			Fix: SummaryFixJSON{Call: n.Call, Args: n.Args, Command: n.Command, Path: n.Path}})
	}
	for _, l := range s.Lines {
		sec := SummarySectionJSON{ID: string(l.Section), Counts: map[string]int{}, Pending: l.Pending, Unread: l.Unread}
		for _, p := range l.Parts {
			if p.Counted && !l.Pending && l.Unread == "" {
				sec.Counts[p.Key] = p.N
			}
		}
		out.Sections = append(out.Sections, sec)
	}
	for _, vm := range s.VMs {
		state := "stopped"
		if vm.Running {
			state = "running"
		}
		out.VMs = append(out.VMs, SummaryVMJSON{Name: vm.Name, State: state})
	}
	return out
}
