package server

import (
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/tui/screens"
	"github.com/jterrazz/studio-station/src/ui/atom"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: SPEC §9.4's keys on a VM's Overview and its SSH: a workspace
// stops after a yes, a server only once its name is typed (SPEC §9.8);
// restart asks nothing; e edits what it was created with; c copies its ssh
// command.
type Screen struct{}

func (Screen) Live() time.Duration { return 0 }

func (Screen) Words(_ screens.Machine, words []string, _ screens.State) []string { return words }

func (Screen) Title(v screens.View) string { return v.Section.Title() + ui.Sep + v.Machine.Label }

func (Screen) Lines(screens.View) ([]string, bool) { return nil, false }

func (Screen) Keys(v screens.View) []ui.Binding {
	copySSH := screens.Bind("c", "copy ssh")
	if v.Section == section.SSH {
		return []ui.Binding{copySSH}
	}
	stop := screens.Bind("s", "stop")
	if v.Machine.Kind == machine.Server {
		stop = screens.Bind("S", "stop… (type the name)")
	}
	return []ui.Binding{stop, screens.Bind("r", "restart"), screens.Bind("e", "edit"), copySSH}
}

// value: a VM's call names its Mac when it is not this one.
func on(m screens.Machine) []string {
	if m.MacHere || m.Mac == "" {
		return nil
	}
	return []string{"--on", m.MacLabel}
}

func (Screen) Press(key string, v screens.View) (screens.Effect, bool) {
	m := v.Machine
	name := m.Name
	if key == "c" {
		return screens.Effect{Call: &screens.Call{Words: []string{"machines", "ssh", name}, Copy: true, Doing: "Reading " + name + "'s ssh command"}}, true
	}
	if v.Section == section.SSH {
		return screens.Effect{}, false
	}
	switch {
	case key == "s" && m.Kind == machine.Workspace:
		return screens.Effect{Call: &screens.Call{Words: append([]string{"machines", "stop", name, "--yes"}, on(m)...),
			Ask:   &screens.Ask{Question: "Stop " + name + "?", Lines: []string{"It is kept, with its disk, until you start or delete it."}, Yes: "stop"},
			Doing: "Stopping " + name, Done: "Stopped " + name}}, true
	case key == "S" && m.Kind == machine.Server:
		return screens.Effect{Call: &screens.Call{Words: append([]string{"machines", "stop", name, "--confirm=" + name}, on(m)...),
			Ask: &screens.Ask{Question: "Stop " + name + "?", Type: name, Yes: "stop",
				Lines: []string{"A server is always on: Stop sets it to Stopped, and nothing starts it again until you do."}},
			Doing: "Stopping " + name, Done: "Stopped " + name}}, true
	case key == "r":
		return screens.Effect{Call: &screens.Call{Words: append([]string{"machines", "restart", name, "--yes"}, on(m)...),
			Doing: "Restarting " + name, Done: "Restarted " + name}}, true
	case key == "e":
		return edit(v), true
	}
	return screens.Effect{}, false
}

func health(p page.Page, id string) string {
	for _, s := range p.Sections {
		for _, it := range s.Items {
			if it.Row != nil && it.Row.ID == id && len(it.Row.Cells) > 0 {
				return atom.Plain(it.Row.Cells[0])
			}
		}
	}
	return ""
}

// invariant: the form starts from what the VM was created with, and the
// call changes only what the answers changed.
func edit(v screens.View) screens.Effect {
	m := v.Machine
	size := strings.Split(health(v.Page, "size"), " · ")
	start := []string{"", "", ""}
	for i, part := range size {
		if i < len(start) {
			start[i] = unit(part)
		}
	}
	fields := []screens.Field{
		{Label: "CPUs", Value: start[0]},
		{Label: "Memory", Value: start[1], Note: "8G, 512M"},
		{Label: "Disk", Value: start[2], Note: "a disk only grows"},
	}
	flags := []string{"--cpus", "--memory", "--disk"}
	if m.Kind == machine.Server {
		check := health(v.Page, "check")
		start = append(start, check)
		fields = append(fields, screens.Field{Label: "Health check", Value: check, Note: "a URL or host:port"})
		flags = append(flags, "--check")
	}
	return screens.Effect{Call: &screens.Call{Doing: "Editing " + m.Name, Done: "Edited " + m.Name + ": it applies when it next starts",
		Form: &screens.Form{Title: "Edit " + m.Name, Fields: fields, Words: func(answers []string) []string {
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
