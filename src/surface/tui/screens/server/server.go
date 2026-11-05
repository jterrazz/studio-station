package server

import (
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type Screen struct{}

func (Screen) Live() time.Duration { return 0 }

func (Screen) Words(_ screen.Machine, words []string, _ screen.State) []string { return words }

func (Screen) Title(v screen.View) string {
	if v.Machine.Kind == machine.Server && v.Section == section.Overview {
		return v.Machine.Label + "  " + kind(v.Machine) + " "
	}
	return v.Section.Title() + ui.Sep + v.Machine.Label
}

func (Screen) Keys(v screen.View) []ui.Binding {
	copySSH := screen.Bind("c", "copy "+strings.ToLower(section.SSH.Title()))
	if v.Section == section.SSH {
		return []ui.Binding{screen.Bind("c", "copy")}
	}
	stop := screen.Bind("s", "stop")
	if v.Machine.Kind == machine.Server {
		stop = screen.Bind("S", "stop"+ui.Ellipsis+" (type the name)")
	}
	return []ui.Binding{stop, screen.Bind("r", "restart"), screen.Bind("e", "edit"), copySSH}
}

// value: a VM's call names its Mac when it is not this one.
func on(m screen.Machine) []string {
	if m.MacHere || m.Mac == "" {
		return nil
	}
	return []string{"--on", m.MacLabel}
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	// invariant: SPEC §9.4, §9.8: a workspace stops after a yes, a server
	// only once its name is typed; restart asks nothing; e edits what it was
	// created with; c copies its ssh command.
	m := v.Machine
	name := m.Name
	if key == "c" {
		return screen.Effect{Call: &screen.Call{Words: append([]string{"machines", string(section.SSH), name}, on(m)...), Copy: true, Doing: "Reading the command that reaches " + name}}, true
	}
	if v.Section == section.SSH {
		return screen.Effect{}, false
	}
	switch {
	case key == "enter" && m.Kind == machine.Server:
		return screen.Effect{Open: &screen.Open{Machine: name, Section: section.Activity}}, true
	case key == "s" && m.Kind == machine.Workspace:
		return screen.Effect{Call: &screen.Call{Words: append([]string{"machines", "stop", name, "--yes"}, on(m)...),
			Ask:   &screen.Ask{Question: "Stop " + name + "?", Lines: []string{"It is kept, with its disk, until you start or delete it."}, Yes: "stop"},
			Doing: "Stopping " + name, Done: "Stopped " + name}}, true
	case key == "S" && m.Kind == machine.Server:
		return screen.Effect{Call: &screen.Call{Words: append([]string{"machines", "stop", name, "--confirm=" + name}, on(m)...),
			Ask: &screen.Ask{Question: "Stop " + name + "?", Type: name, Yes: "stop",
				Lines: []string{"A server is always on: Stop sets it to Stopped, and nothing starts it again until you do."}},
			Doing: "Stopping " + name, Done: "Stopped " + name}}, true
	case key == "r":
		return screen.Effect{Call: &screen.Call{Words: append([]string{"machines", "restart", name, "--yes"}, on(m)...),
			Doing: "Restarting " + name, Done: "Restarted " + name}}, true
	case key == "e":
		return edit(v), true
	}
	return screen.Effect{}, false
}

func health(p page.Page, id string) string {
	if r, ok := row(p, "health", id); ok && len(r.Cells) > 0 {
		return ui.Plain(r.Cells[0])
	}
	return ""
}

func row(p page.Page, sec, id string) (page.Row, bool) {
	for _, s := range p.Sections {
		if sec != "" && s.ID != sec {
			continue
		}
		for _, it := range s.Items {
			if it.Row != nil && it.Row.ID == id {
				return *it.Row, true
			}
		}
	}
	return page.Row{}, false
}

// invariant: the form starts from what the VM was created with, and the
// call changes only what the answers changed.
func edit(v screen.View) screen.Effect {
	m := v.Machine
	size := strings.Split(health(v.Page, "size"), ui.Sep)
	start := []string{"", "", ""}
	for i, part := range size {
		if i < len(start) {
			start[i] = unit(part)
		}
	}
	fields := []screen.Field{
		{Label: "CPUs", Value: start[0]},
		{Label: "Memory", Value: start[1], Note: "8G, 512M"},
		{Label: "Disk", Value: start[2], Note: "a disk only grows"},
	}
	flags := []string{"--cpus", "--memory", "--disk"}
	if m.Kind == machine.Server {
		check := health(v.Page, "check")
		start = append(start, check)
		fields = append(fields, screen.Field{Label: "Health check", Value: check, Note: "a URL or host:port"})
		flags = append(flags, "--check")
	}
	return screen.Effect{Call: &screen.Call{Doing: "Editing " + m.Name, Done: "Edited " + m.Name + ": it applies when it next starts",
		Form: &screen.Form{Title: "Edit " + m.Name, Fields: fields, Words: func(answers []string) []string {
			words := []string{"machines", "edit", m.Name}
			for i, a := range answers {
				if a = strings.TrimSpace(a); a != start[i] && a != "" {
					words = append(words, flags[i], a)
				}
			}
			return append(words, on(m)...)
		}}}}
}

// value: a size as the edit's flags take it: `4 CPU` is 4, `8 GB` is 8G.
func unit(s string) string {
	s = strings.TrimSpace(s)
	if n, ok := strings.CutSuffix(s, " CPU"); ok {
		return n
	}
	for _, u := range []string{"GB", "MB", "TB"} {
		if n, ok := strings.CutSuffix(s, " "+u); ok {
			return n + u[:1]
		}
	}
	return s
}
