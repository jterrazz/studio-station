package ports

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/screen"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: the sections of the Ports page by class (SPEC §5.6): what needs
// you, the declared public ports, the shared private ones.
const (
	needsYou = "needs-you"
	public   = "public"
	private  = "private"
)

type Screen struct{}

func (Screen) Live() time.Duration { return 0 }

func (Screen) Words(_ screen.Machine, words []string, _ screen.State) []string { return words }

func (Screen) Title(v screen.View) string {
	n := 0
	for _, s := range v.Page.Sections {
		if slices.Contains([]string{needsYou, public, private}, s.ID) {
			n += len(s.Items)
		}
	}
	where := v.Machine.Label
	if v.Machine.Kind == "" {
		where = "all machines"
	}
	return "Ports" + ui.Sep + where + ui.Sep + strconv.Itoa(n)
}

func (Screen) Lines(screen.View) ([]string, bool) { return nil, false }

func (Screen) Keys(v screen.View) []ui.Binding {
	var keys []ui.Binding
	switch v.Of.ID {
	case needsYou:
		keys = append(keys, screen.Bind("d", "declare public"))
	case public:
		keys = append(keys, screen.Bind("u", "undeclare"))
	case private:
		keys = append(keys, screen.Bind("x", "stop sharing"), screen.Bind("c", "copy the link"))
	}
	if sharer(v.Machine) {
		keys = append(keys, screen.Bind("s", "share"))
	}
	return keys
}

// value: a VM's Ports shares one of its ports; All machines asks which VM.
func sharer(m screen.Machine) bool {
	return m.Kind == "" || m.Kind == machine.Workspace || m.Kind == machine.Server
}

func port(v screen.View) (name, number string, ok bool) {
	if !v.HasRow {
		return "", "", false
	}
	name, number, ok = strings.Cut(v.Row.ID, ":")
	return name, number, ok
}

func (Screen) Press(key string, v screen.View) (screen.Effect, bool) {
	// invariant: SPEC §9.4: each key on the row it acts on; enter opens the
	// row's detail, with its QR code, as every list does.
	name, number, onPort := port(v)
	what := name + " " + number
	switch {
	case key == "c" && onPort && v.Of.ID == private:
		link := ui.Plain(screen.Cell(v.Of, v.Row, "url"))
		if link == "" {
			return screen.Effect{Say: what + " has no link to copy"}, true
		}
		return screen.Effect{Copy: link}, true
	case key == "d" && onPort && v.Of.ID == needsYou:
		return screen.Effect{Call: &screen.Call{Doing: "Declaring " + what + " public", Done: "Declared " + what + " public",
			Form: &screen.Form{Title: "Declare " + what + " public", Fields: []screen.Field{{Label: "Route", Value: "lan", Note: "the route that reaches it, as --via names it"}},
				Words: func(answers []string) []string {
					return []string{"ports", "declare", name, number, "--via", strings.TrimSpace(answers[0])}
				}}}}, true
	case key == "u" && onPort && v.Of.ID == public:
		return screen.Effect{Call: &screen.Call{Words: []string{"ports", "undeclare", name, number, "--yes"},
			Ask:   &screen.Ask{Question: "Undeclare " + what + "?", Lines: []string{"It needs you again while it is reachable from outside."}, Yes: "undeclare"},
			Doing: "Undeclaring " + what, Done: "Undeclared " + what}}, true
	case key == "x" && onPort && v.Of.ID == private:
		return screen.Effect{Call: &screen.Call{Words: []string{"ports", "stop-sharing", name, number, "--yes"},
			Ask:   &screen.Ask{Question: "Stop sharing " + what + "?", Lines: []string{"Its reserved port is released; it is not shared again until you share it."}, Yes: "stop sharing"},
			Doing: "Stopping the sharing of " + what, Done: "Stopped sharing " + what}}, true
	case key == "s" && sharer(v.Machine):
		return share(v.Machine), true
	}
	return screen.Effect{}, false
}

func share(m screen.Machine) screen.Effect {
	fields := []screen.Field{{Label: "Port", Note: "the port it listens on inside"}}
	if m.Kind == "" {
		fields = append([]screen.Field{{Label: "VM", Note: "the VM whose port it is"}}, fields...)
	}
	title := "Share a port"
	if m.Kind != "" {
		title += " of " + m.Label
	}
	return screen.Effect{Call: &screen.Call{Doing: "Sharing a port", Done: "Shared it",
		Form: &screen.Form{Title: title, Fields: fields, Words: func(answers []string) []string {
			vm := m.Name
			if m.Kind == "" {
				vm, answers = strings.TrimSpace(answers[0]), answers[1:]
			}
			return []string{"ports", "share", vm, strings.TrimSpace(answers[0])}
		}}}}
}
