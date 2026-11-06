package server

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

const (
	activity = "activity"
	sshRead  = "door"
	ports    = "ports"
	overview = "overview"
	macSSH   = "mac-ssh"
)

func (Screen) Reads(m screen.Machine, id section.ID, words func(section.ID) []string) []screen.Read {
	// invariant: SPEC §9.2: a server's Overview reads, beside its own page,
	// its Activity over the last hour and its Mac's Ports (a server's ports
	// are its Mac's to list); its SSH screen reads its Overview (its
	// address) and the ssh command of its Mac (the hop's login).
	if m.Kind != machine.Server {
		return nil
	}
	if id == section.Overview {
		portsOn := []string{"ports"}
		if !m.MacHere {
			portsOn = append(portsOn, "--on", m.Mac)
		}
		return []screen.Read{{Name: activity, Words: append(words(section.Activity), "--graph", "1h")}, {Name: ports, Words: portsOn}, {Name: sshRead, Words: words(section.SSH)}}
	}
	if id == section.SSH {
		reads := []screen.Read{{Name: overview, Words: words(section.Overview)}}
		if !m.MacHere {
			reads = append(reads, screen.Read{Name: macSSH, Words: []string{"machines", string(section.SSH), m.Mac}})
		}
		return reads
	}
	return nil
}

func (Screen) Paned(m screen.Machine, id section.ID) bool {
	return m.Kind == machine.Server && id == section.Overview
}

func caption(p page.Page) []page.Value {
	for _, s := range p.Sections {
		if s.ID == "overview" {
			return s.Caption
		}
	}
	return nil
}

func running(p page.Page) bool {
	for _, c := range caption(p) {
		if c.Label != "" {
			return page.Mark(c.Label) == page.OK
		}
	}
	return false
}

func (Screen) Lines(v screen.View) ([]string, bool) {
	if v.Machine.Kind != machine.Server {
		return nil, false
	}
	if v.Section == section.SSH {
		return door(v), true
	}
	m, p := v.Machine, v.Page
	sv := ui.Server{Mark: page.Absent, Width: v.Width, Dark: v.Dark, Unmanaged: "Apps, Settings, Disk, Accounts"}
	for _, c := range caption(p) {
		if c.Label != "" {
			sv.Mark = page.Mark(c.Label)
			sv.State = append(sv.State, c.Text)
		}
	}
	if act, ok := v.Beside[activity]; ok {
		if r, ok := row(act, "machine", "machine"); ok && len(r.Cells) > 0 {
			for _, w := range r.Cells[0].Items {
				if t := ui.Plain(w); strings.HasPrefix(t, "up ") {
					sv.State = append(sv.State, t)
				}
			}
		}
		for _, id := range []string{"cpu", "memory", "disk-free"} {
			if r, ok := row(act, "graphs", id); ok {
				r.Label = map[string]string{"cpu": "CPU", "memory": "Mem", "disk-free": "Disk"}[id]
				sv.Graphs = append(sv.Graphs, r)
			}
		}
	}
	if r, ok := row(p, "health", "restarts"); ok && len(r.Cells) > 0 {
		sv.Facts = append(sv.Facts, ui.Pair{Label: r.Label, Text: ui.Plain(r.Cells[0])})
	}
	wanted := "Running" + ui.Sep + "Stop sets it to Stopped"
	if !running(p) {
		wanted = "Stopped" + ui.Sep + "Start sets it to Running"
	}
	sv.Facts = append(sv.Facts, ui.Pair{Label: "Wanted", Text: wanted})
	sv.Created = append(sv.Created, ui.Pair{Label: "Kind", Text: kind(m) + ui.Sep + "always on"})
	if size := health(p, "size"); size != "" {
		sv.Created = append(sv.Created, ui.Pair{Label: "Size", Text: size})
	}
	if check := health(p, "check"); check != "" {
		sv.Created = append(sv.Created, ui.Pair{Label: "Check", Text: check})
	}
	sv.SSH = command(v.Beside[sshRead], m.Name)
	sv.Keys = []ui.Pair{{Label: "r", Text: "Restart"}, {Label: "S", Text: "Stop" + ui.Ellipsis + " type " + m.Name}}
	return sv.Lines(), true
}

func kind(m screen.Machine) string {
	k := string(m.Kind)
	if k == "" {
		return k
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// value: the ssh command `machines ssh` prints for a machine.
func command(p page.Page, name string) string {
	for _, s := range p.Sections {
		for _, it := range s.Items {
			if it.Row != nil && it.Row.ID == name && len(it.Row.Cells) > 0 {
				return ui.Plain(it.Row.Cells[0])
			}
		}
	}
	return ""
}

func (Screen) Panes(v screen.View) []ui.Panel {
	p := v.Page
	var checks []ui.Check
	worst := page.OK
	for _, id := range []string{"running", "address", string(section.SSH), "check"} {
		r, ok := row(p, "health", id)
		if !ok {
			continue
		}
		c := ui.Check{Mark: r.Mark, Label: r.Label}
		if len(r.Cells) > 0 {
			c.Text = ui.Plain(r.Cells[0])
		}
		if id == "check" {
			c.Under, c.Text = c.Text, ""
		}
		if r.Mark != page.OK {
			worst = r.Mark
		}
		checks = append(checks, c)
	}
	return []ui.Panel{{Title: "Health " + ui.Glyph(worst), Lines: ui.Checks(v.Dark, checks)}, portsPane(v)}
}

// value: the server's ports, from its Mac's Ports page: Public with the
// route each was declared by, Private with what listens, then its own
// tailnet device when it has one.
func portsPane(v screen.View) ui.Panel {
	pp, ok := v.Beside[ports]
	if !ok {
		return ui.Panel{Title: "Ports", Lines: []string{" " + ui.Glyph(page.Awaiting) + " reading"}}
	}
	n := 0
	var groups []ui.PortGroup
	tidy := true
	for _, s := range pp.Sections {
		if s.ID != "public" && s.ID != "private" {
			if s.ID == "summary" {
				for _, it := range s.Items {
					if it.Row != nil && it.Row.Mark != page.OK {
						tidy = false
					}
				}
			}
			continue
		}
		g := ui.PortGroup{Title: map[string]string{"public": "Public" + ui.Sep + "declared", "private": "Private" + ui.Sep + catalog.TailnetProduct}[s.ID]}
		for _, it := range s.Items {
			r := it.Row
			if r == nil || r.Label != v.Machine.Name {
				continue
			}
			port := ui.Plain(screen.Cell(s, *r, "port"))
			what := screen.Cell(s, *r, "process")
			mark := page.Unjudged
			if s.ID == "public" {
				what, mark = screen.Cell(s, *r, "declared"), r.Mark
				if len(what.Items) > 0 {
					what = what.Items[0]
				}
			}
			if state := ui.Plain(screen.Cell(s, *r, "state")); strings.HasPrefix(state, "own ") {
				if _, device, ok := strings.Cut(state, ": "); ok {
					g.Note = "on its own device: " + device
				}
			}
			g.Ports = append(g.Ports, ui.Check{Mark: mark, Label: port, Text: ui.Plain(what)})
			n++
		}
		if len(g.Ports) > 0 {
			groups = append(groups, g)
		}
	}
	var ending []string
	if n == 0 {
		ending = append(ending, "Nothing shared.")
	}
	if tidy {
		ending = append(ending, "Nothing else reachable", "outside "+catalog.TailnetProduct+". "+ui.Glyph(page.OK))
	}
	return ui.Panel{Title: "Ports" + ui.Sep + strconv.Itoa(n), Lines: ui.Ports(v.Dark, groups, ending)}
}

// value: SPEC §9.2's SSH screen: the command the section's page prints,
// and each hop from This Mac to the server.
func door(v screen.View) []string {
	m := v.Machine
	d := ui.Door{Name: m.Name, Mac: m.MacLabel, MacHere: m.MacHere, Server: true, Dark: v.Dark}
	d.Command = command(v.Page, m.Name)
	if !m.MacHere {
		hop := "over " + catalog.TailnetProduct
		if user, _, ok := strings.Cut(strings.TrimPrefix(command(v.Beside[macSSH], m.Mac), string(section.SSH)+" "), "@"); ok {
			hop += ", as " + user
		}
		d.Reached = append(d.Reached, ui.Pair{Label: "This Mac" + ui.Crumb + m.MacLabel, Text: hop})
	}
	vm := ui.Pair{Label: m.MacLabel + ui.Crumb + m.Name, Text: "its Mac's private network"}
	if ov, ok := v.Beside[overview]; ok {
		if ip := health(ov, "address"); ip != "" && ip != "failing" {
			vm.Text = ip + ui.Sep + vm.Text
		}
	}
	d.Reached = append(d.Reached, vm)
	return d.Lines()
}
