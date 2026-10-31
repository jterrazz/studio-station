package status

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/present"
	presentconfig "github.com/jterrazz/studio-station/src/present/config"
	presenttailnet "github.com/jterrazz/studio-station/src/present/tailnet"
	presenttools "github.com/jterrazz/studio-station/src/present/tools"
)

func Snapshot(s status.Snapshot, context string) present.Answer {
	blocks := []present.Block{
		section("Machine", machineRows(s)),
		section("Environment", environmentRows(s)),
		section("Workspace", workspaceRows(s.Workspace)),
		section("Identity", checkRows(s.Identity)),
	}
	if len(s.Daemons) > 0 {
		blocks = append(blocks, section("Daemons", checkRows(s.Daemons)))
	}
	if len(s.Services) > 0 {
		blocks = append(blocks, section("Services", checkRows(s.Services)))
	}
	blocks = append(blocks,
		present.Section{Title: "Tools", Counts: presenttools.Counts(s.Tools), Blocks: []present.Block{present.Table{Rows: presenttools.Rows(notOK(s.Tools))}}},
		present.Section{Title: "Config", Counts: presentconfig.Counts(s.Config), Blocks: []present.Block{present.Table{Rows: presentconfig.Rows(notOKConfig(s.Config))}}},
	)
	return present.Answer{
		Doc: present.Document{
			Command: "station status",
			Context: context,
			Blocks:  blocks,
			Hints:   []string{"station tools for the catalogue", "station config for the items"},
		},
		JSON: jsonOf(s),
	}
}

func section(title string, rows []present.Row) present.Section {
	return present.Section{Title: title, Blocks: []present.Block{present.Table{Rows: rows}}}
}

func notOK(reports []tools.Report) []tools.Report {
	var rest []tools.Report
	for _, r := range reports {
		if r.State != catalog.StateOK {
			rest = append(rest, r)
		}
	}
	return rest
}

func notOKConfig(reports []config.Report) []config.Report {
	var rest []config.Report
	for _, r := range reports {
		if r.State != catalog.StateOK {
			rest = append(rest, r)
		}
	}
	return rest
}

func row(mark present.Mark, name, value, detail string) present.Row {
	return present.Row{Mark: mark, Cells: []present.Cell{
		{Text: name, Tone: present.Strong},
		{Text: value},
		{Text: detail, Tone: present.Muted},
	}}
}

func checkRows(checks []status.Check) []present.Row {
	rows := make([]present.Row, 0, len(checks))
	for _, c := range checks {
		value := c.Value
		if value == "" && c.State == catalog.StateUnknown {
			value = "unknown"
		}
		rows = append(rows, row(present.MarkOf(c.State), strings.ReplaceAll(c.Name, "-", " "), value, c.Detail))
	}
	return rows
}

func machineRows(s status.Snapshot) []present.Row {
	m := s.Machine
	var parts []string
	for _, part := range []string{string(m.Kind), strings.TrimSpace(m.OS + " " + m.Arch), m.User, m.Shell} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	facts := strings.Join(parts, " · ")
	name := row(present.MarkOK, "name", m.Name, facts)
	if !m.Registered {
		name = row(present.MarkAbsent, "name", "unregistered", facts)
	}
	rows := []present.Row{name}
	if s.System.Uptime != "" {
		detail := ""
		if s.System.Battery != "" {
			detail = "battery " + s.System.Battery
		}
		rows = append(rows, row(present.MarkOK, "uptime", s.System.Uptime, detail))
	}
	return append(rows, setupRow(s.Setup))
}

func setupRow(s status.Setup) present.Row {
	for _, step := range s.Steps {
		if step.State != catalog.StateOK {
			return row(present.MarkOf(s.State), "setup", step.Name, "station setup")
		}
	}
	if s.State != catalog.StateOK {
		return row(present.MarkOf(s.State), "setup", "unknown", "station setup")
	}
	return row(present.MarkOK, "setup", "done", "")
}

func environmentRows(s status.Snapshot) []present.Row {
	rows := checkRows(s.Environment.Network)
	rows = append(rows, checkRows(s.Environment.Security)...)
	rows = append(rows, checkRows(s.Environment.Updates)...)
	return append(rows, presenttailnet.Rows(s.Tailnet)[0])
}

func workspaceRows(w status.Workspace) []present.Row {
	return []present.Row{reposRow(w.Repositories), depsRow(w.Dependencies), dockerRow(w.Docker), diskRow(w.Disk)}
}

func reposRow(repos []status.Repository) present.Row {
	if len(repos) == 0 {
		return row(present.MarkAbsent, "repos", "none", "~/Developer")
	}
	dirty := 0
	for _, r := range repos {
		dirty += r.Dirty
	}
	return row(present.MarkOK, "repos", strconv.Itoa(len(repos)), strconv.Itoa(dirty)+" dirty")
}

func depsRow(deps []status.Dependency) present.Row {
	if len(deps) == 0 {
		return row(present.MarkAbsent, "deps", "none", "")
	}
	outdated, repos, unscanned := 0, 0, 0
	for _, d := range deps {
		switch {
		case d.Outdated < 0:
			unscanned++
		case d.Outdated > 0:
			outdated += d.Outdated
			repos++
		}
	}
	detail := ""
	if unscanned > 0 {
		detail = strconv.Itoa(unscanned) + " not scanned in time"
	}
	if outdated == 0 {
		return row(present.MarkOK, "deps", "up to date", detail)
	}
	in := "in " + strconv.Itoa(repos) + " repos"
	if detail != "" {
		in += " · " + detail
	}
	return row(present.MarkDegraded, "deps", strconv.Itoa(outdated)+" outdated", in)
}

func dockerRow(d status.Docker) present.Row {
	switch d.State {
	case catalog.StateOK:
		return row(present.MarkOK, "docker", "running", strconv.Itoa(d.Containers)+" containers · "+strconv.Itoa(d.Images)+" images")
	case catalog.StateAbsent:
		return row(present.MarkAbsent, "docker", "not installed", "station tools install docker")
	case catalog.StateDegraded:
		return row(present.MarkDegraded, "docker", "not running", "")
	case catalog.StateBroken, catalog.StateUnknown:
	}
	return row(present.MarkDegraded, "docker", "unknown", "")
}

func diskRow(disk []status.Disk) present.Row {
	var total int64
	for _, d := range disk {
		total += d.Bytes
	}
	if total == 0 {
		return row(present.MarkOK, "disk", "no caches", "")
	}
	return row(present.MarkOK, "disk", present.Bytes(total), "reclaimable: station clean")
}
