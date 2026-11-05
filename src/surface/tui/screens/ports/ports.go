package ports

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/surface/tui/screens"
	"github.com/jterrazz/studio-station/src/ui/atom"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

// value: the sections of the Ports page by class (SPEC §5.6): what needs
// you, the declared public ports, the shared private ones.
const (
	needsYou = "needs-you"
	public   = "public"
	private  = "private"
)

// value: the routes a public port is declared by.
var routes = []string{"cloudflare-tunnel", "tailscale-funnel", "lan"}

type Screen struct{}

func (Screen) Live() time.Duration { return 0 }

func (Screen) Words(_ screens.Machine, words []string, _ screens.State) []string { return words }

func (Screen) Title(v screens.View) string {
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

func (Screen) Lines(screens.View) ([]string, bool) { return nil, false }

func (Screen) Keys(v screens.View) []ui.Binding {
	var keys []ui.Binding
	switch v.Of.ID {
	case needsYou:
		keys = append(keys, screens.Bind("d", "declare public"))
	case public:
		keys = append(keys, screens.Bind("u", "undeclare"))
	case private:
		keys = append(keys, screens.Bind("x", "stop sharing"), screens.Bind("c", "copy the link"))
	}
	if sharer(v.Machine) {
		keys = append(keys, screens.Bind("s", "share"))
	}
	return keys
}

// value: a VM's Ports shares one of its ports; All machines asks which VM.
func sharer(m screens.Machine) bool {
	return m.Kind == "" || m.Kind == machine.Workspace || m.Kind == machine.Server
}

func port(v screens.View) (name, number string, ok bool) {
	if !v.HasRow {
		return "", "", false
	}
	name, number, ok = strings.Cut(v.Row.ID, ":")
	return name, number, ok
}

func (Screen) Press(key string, v screens.View) (screens.Effect, bool) {
	// invariant: SPEC §9.4: each key on the row it acts on; enter opens the
	// row's detail, with its QR code, as every list does.
	name, number, onPort := port(v)
	what := name + " " + number
	switch {
	case key == "c" && onPort && v.Of.ID == private:
		link := atom.Plain(screens.Cell(v.Of, v.Row, "url"))
		if link == "" {
			return screens.Effect{Say: what + " has no link to copy"}, true
		}
		return screens.Effect{Copy: link}, true
	case key == "d" && onPort && v.Of.ID == needsYou:
		return screens.Effect{Call: &screens.Call{Doing: "Declaring " + what + " public", Done: "Declared " + what + " public",
			Form: &screens.Form{Title: "Declare " + what + " public", Fields: []screens.Field{{Label: "Route", Value: "lan", Note: strings.Join(routes, ", ")}},
				Words: func(answers []string) []string {
					return []string{"ports", "declare", name, number, "--via", strings.TrimSpace(answers[0])}
				}}}}, true
	case key == "u" && onPort && v.Of.ID == public:
		return screens.Effect{Call: &screens.Call{Words: []string{"ports", "undeclare", name, number, "--yes"},
			Ask:   &screens.Ask{Question: "Undeclare " + what + "?", Lines: []string{"It is red again while it listens where something outside Tailscale reaches it."}, Yes: "undeclare"},
			Doing: "Undeclaring " + what, Done: "Undeclared " + what}}, true
	case key == "x" && onPort && v.Of.ID == private:
		return screens.Effect{Call: &screens.Call{Words: []string{"ports", "stop-sharing", name, number, "--yes"},
			Ask:   &screens.Ask{Question: "Stop sharing " + what + "?", Lines: []string{"Its reserved port is released; it is not shared again until you share it."}, Yes: "stop sharing"},
			Doing: "Stopping the sharing of " + what, Done: "Stopped sharing " + what}}, true
	case key == "s" && sharer(v.Machine):
		return share(v.Machine), true
	}
	return screens.Effect{}, false
}

func share(m screens.Machine) screens.Effect {
	fields := []screens.Field{{Label: "Port", Note: "the port it listens on inside"}}
	if m.Kind == "" {
		fields = append([]screens.Field{{Label: "VM", Note: "the VM whose port it is"}}, fields...)
	}
	title := "Share a port"
	if m.Kind != "" {
		title += " of " + m.Label
	}
	return screens.Effect{Call: &screens.Call{Doing: "Sharing a port", Done: "Shared it on Tailscale",
		Form: &screens.Form{Title: title, Fields: fields, Words: func(answers []string) []string {
			vm := m.Name
			if m.Kind == "" {
				vm, answers = strings.TrimSpace(answers[0]), answers[1:]
			}
			return []string{"ports", "share", vm, strings.TrimSpace(answers[0])}
		}}}}
}
