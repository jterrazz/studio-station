package activity

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type window struct {
	flag, words string
}

// value: SPEC §6.4's picker, `w` cycling it: 2 s points, then minutes.
func windows() []window { return []window{{"6m", "6 min"}, {"1h", "1 h"}, {"24h", "24 h"}} }

type Screen struct{}

func (Screen) Live() time.Duration { return 2 * time.Second }

func of(st screen.State) window {
	for _, w := range windows() {
		if w.flag == st["window"] {
			return w
		}
	}
	return windows()[0]
}

func (Screen) Words(m screen.Machine, words []string, st screen.State) []string {
	if m.Kind == "" {
		return words
	}
	return append(slices.Clone(words), "--graph", of(st).flag)
}

func banner(p page.Page) []string {
	for _, s := range p.Sections {
		if s.Role == page.Banner && s.ID != page.OnSection {
			for _, it := range s.Items {
				if it.Row != nil && len(it.Row.Cells) > 0 {
					var words []string
					for _, w := range it.Row.Cells[0].Items {
						words = append(words, ui.Plain(w))
					}
					if len(words) == 0 {
						words = append(words, ui.Plain(it.Row.Cells[0]))
					}
					return words
				}
			}
		}
	}
	return nil
}

func (Screen) Title(v screen.View) string {
	if v.Machine.Kind == "" {
		n := 0
		for _, s := range v.Page.Sections {
			if s.ID == "machines" {
				n = len(s.Items)
			}
		}
		return "Activity" + ui.Sep + "all machines" + ui.Sep + strconv.Itoa(n)
	}
	title := "Activity" + ui.Sep + v.Machine.Label
	for _, w := range banner(v.Page) {
		if strings.HasPrefix(w, "from ") {
			return title + ui.Sep + w
		}
	}
	return title + ui.Sep + "last " + of(v.State).words
}

func (Screen) Keys(v screen.View) []ui.Binding {
	if v.Machine.Kind == "" {
		return []ui.Binding{screen.Bind("enter", "open its Activity")}
	}
	return []ui.Binding{screen.Bind("w", "window"), screen.Bind("p", "sort processes")}
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	switch {
	case v.Machine.Kind == "" && key == "enter" && v.HasRow && v.Of.ID == "machines":
		return screen.Effect{Open: &screen.Open{Machine: v.Row.ID, Section: section.Activity}}, true
	case v.Machine.Kind == "":
		return screen.Effect{}, false
	case key == "w":
		all := windows()
		i := slices.Index(all, of(v.State))
		return screen.Effect{State: v.State.With("window", all[(i+1)%len(all)].flag), Read: true}, true
	case key == "p":
		sort := "memory"
		if v.State["sort"] == "memory" {
			sort = "cpu"
		}
		return screen.Effect{State: v.State.With("sort", sort)}, true
	}
	return screen.Effect{}, false
}

func find(p page.Page, id string) (page.Section, bool) {
	for _, s := range p.Sections {
		if s.ID == id {
			return s, true
		}
	}
	return page.Section{}, false
}

func (Screen) Lines(v screen.View) ([]string, bool) {
	graphs, ok := find(v.Page, pageactivity.Graphs)
	if v.Machine.Kind == "" || !ok {
		return nil, false
	}
	a := ui.Activity{Banner: banner(v.Page), Graphs: graphs, Sort: v.State["sort"], Width: v.Width, Dark: v.Dark, Sampled: sampled(v)}
	a.Values, _ = find(v.Page, "values")
	if procs, ok := find(v.Page, pageactivity.Processes); ok {
		a.Processes = &procs
	}
	return a.Lines(), true
}

// value: SPEC §9.2: a machine of this Mac is sampled by this Mac's engine,
// every 2 s while the TUI shows it, so its 6 min window is drawn from 2 s
// points and its longer ones from 60 s points; another Mac's machine is
// that Mac's engine's to sample, and its banner stays.
func sampled(v screen.View) string {
	m := v.Machine
	if !m.Here && !m.MacHere {
		return ""
	}
	if of(v.State).flag == windows()[0].flag {
		return "Sampled every 2 s by the engine while you watch"
	}
	return "Sampled every 60 s by the engine"
}
