package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const (
	serverLabel = 11
	serverGraph = 10
)

type Pair struct{ Label, Text string }

type Server struct {
	// value: a server's Overview (SPEC §9.2): its state line, its facts,
	// what it was created with, its activity over the last hour, its one
	// door, what station leaves to its own tooling, and the keys it takes.
	Mark      page.Mark
	State     []string
	Facts     []Pair
	Created   []Pair
	Graphs    []page.Row
	SSH       string
	Unmanaged string
	Keys      []Pair
	Width     int
	Dark      bool
}

func (s Server) Lines() []string {
	st := token.New(s.Dark)
	out := []string{" " + atom.Mark(st, s.Mark) + strings.Join(s.State, token.Sep)}
	for _, f := range s.Facts {
		out = append(out, " "+atom.Pad(f.Label, serverLabel)+f.Text)
	}
	head := func(title, key string) string {
		line := " " + st.Strong.Render(title)
		if key == "" {
			return line
		}
		return atom.Pad(line, max(s.Width-atom.Width(key)-1, atom.Width(line)+2)) + st.Muted.Render(key)
	}
	out = append(out, "", head("CREATED WITH", "e edit"))
	for _, c := range s.Created {
		out = append(out, "   "+atom.Pad(c.Label, 9)+c.Text)
	}
	if len(s.Graphs) > 0 {
		out = append(out, "", head("ACTIVITY"+token.Sep+"last hour", "enter open"))
		for _, r := range s.Graphs {
			out = append(out, "   "+atom.Pad(r.Label, 5)+Graph(r, serverGraph)+"  "+GraphText(r))
		}
	}
	if s.SSH != "" {
		out = append(out, "", head("SSH"+token.Sep+"for debugging", ""), "   "+s.SSH+"   "+st.Muted.Render("c copy"))
	}
	if s.Unmanaged != "" {
		out = append(out, "", " "+st.Strong.Render("NOT MANAGED: ")+s.Unmanaged+".", " Your own tooling manages what runs inside.")
	}
	if len(s.Keys) > 0 {
		keys := make([]string, 0, len(s.Keys))
		for _, k := range s.Keys {
			keys = append(keys, st.Accent.Render(k.Label)+" "+k.Text)
		}
		out = append(out, "", "   "+strings.Join(keys, "   "))
	}
	return out
}

func Graph(r page.Row, width int) string {
	// value: a measure's graph at a width: the bar of a share, else the
	// sparkline of its points from its label, the newest at its right end.
	if len(r.Cells) < 2 {
		return atom.Spaces(width)
	}
	g := r.Cells[1]
	if g.Kind == page.KindRatio {
		return atom.Bar(g.N, g.Of, width)
	}
	samples := make([]int, 0, len(g.Items))
	for _, x := range g.Items[min(1, len(g.Items)):] {
		samples = append(samples, int(x.N))
	}
	return atom.Pad(atom.Graph(samples, width), width)
}

func GraphText(r page.Row) string {
	// value: what a short graph line says beside it: the measure's first
	// words, or, for a share, how much is used of how much.
	if len(r.Cells) > 1 && r.Cells[1].Kind == page.KindRatio {
		used, of := atom.Bytes(r.Cells[1].N), atom.Bytes(r.Cells[1].Of)
		u, unit, _ := strings.Cut(used, " ")
		o, total, _ := strings.Cut(of, " ")
		u, o = strings.TrimSuffix(u, ".0"), strings.TrimSuffix(o, ".0")
		if unit == total {
			return u + " of " + o + " " + total
		}
		return u + " " + unit + " of " + o + " " + total
	}
	if len(r.Cells) == 0 {
		return ""
	}
	c := r.Cells[0]
	if len(c.Items) > 0 {
		c = c.Items[0]
	}
	return atom.Plain(c)
}

type Door struct {
	// value: a server's SSH screen (SPEC §9.2): its one door, how each hop
	// reaches it, and what station leaves to its own tooling.
	Name, Command, Mac string
	MacHere            bool
	Reached            []Pair
	Server             bool
	Dark               bool
}

func (d Door) Lines() []string {
	st := token.New(d.Dark)
	through := ""
	if !d.MacHere && d.Mac != "" {
		through = ", through " + d.Mac
	}
	out := []string{" The one door into " + d.Name + through + ". No password: your key.", ""}
	out = append(out, "   "+atom.Pad(d.Command, 48)+st.Muted.Render("c copy"))
	if through != "" {
		out = append(out, "   From any Mac: the block in ~/.ssh/config hops through "+d.Mac+".")
	}
	out = append(out, "   In an editor: Remote SSH to "+d.Name+" (Zed, VS Code).", "", " "+st.Strong.Render("HOW IT IS REACHED"))
	for _, r := range d.Reached {
		out = append(out, "   "+atom.Pad(r.Label, 22)+r.Text)
	}
	out = append(out, "", " Files: scp or rsync through the same name. No folder of a Mac is shared.")
	if d.Server {
		out = append(out, "", " Station opens it for debugging only; your tooling owns what runs inside.")
	}
	return out
}

type Check struct {
	// value: a health step: its mark, what it checks, what it found, and a
	// line under it (the URL a check asks).
	Mark               page.Mark
	Label, Text, Under string
}

func Checks(dark bool, checks []Check) []string {
	st := token.New(dark)
	out := make([]string, 0, len(checks))
	for _, c := range checks {
		line := " " + atom.Mark(st, c.Mark) + c.Label
		if c.Text != "" {
			line += "  " + c.Text
		}
		out = append(out, line)
		if c.Under != "" {
			out = append(out, "   "+c.Under)
		}
	}
	return out
}

type PortGroup struct {
	// value: the ports of one class a machine has, each by its number and
	// what it is, then a line under them.
	Title string
	Ports []Check
	Note  string
}

func Ports(dark bool, groups []PortGroup, ending []string) []string {
	st := token.New(dark)
	var out []string
	for _, g := range groups {
		out = append(out, " "+st.Strong.Render(g.Title))
		for _, p := range g.Ports {
			line := "   " + atom.Pad(p.Label, 5) + p.Text
			if p.Mark != page.Unjudged {
				line += " " + atom.Tone(st, p.Mark).Render(atom.Glyph(p.Mark))
			}
			out = append(out, line)
		}
		if g.Note != "" {
			out = append(out, "   "+g.Note)
		}
		out = append(out, "")
	}
	for _, e := range ending {
		out = append(out, " "+e)
	}
	return out
}
