package menubar

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	pageports "github.com/jterrazz/studio-station/src/pages/ports"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	PortsID page.ID = "menubar.ports"

	ActionReview        = "review"
	ActionDeclarePublic = "declare-public"
	ActionSharePort     = "share-port"
	ActionStopSharing   = "stop-sharing:"

	CallDeclare     = "ports.declare"
	CallShare       = "ports.share"
	CallStopSharing = "ports.stop-sharing"
)

type PortsOf struct {
	// value: the machine whose ports the page lists, as the owner names it
	// (`mac-mini`, `linux-1`); empty is This Mac.
	Machine string
	// value: the machine is a workspace, whose page shares its own ports.
	Workspace bool
}

func Ports(l ports.Listing, of PortsOf, here page.Context) page.Page {
	// invariant: SPEC §10.2 Ports and A port: the ports page of §5.6, red on
	// top, then Public, then Private, each row's class in words; every action
	// it shows is a call, but Review, which opens a terminal on the same list.
	p := pageports.Listed(l, here, false)
	p.ID = PortsID
	p.Actions = nil
	if of.Machine != "" && len(p.Sections) > 0 && p.Sections[0].ID == "summary" {
		p.Sections[0].Items[0].Row.Label = of.Machine
	}
	var red []string
	for _, s := range p.Sections {
		for _, it := range s.Items {
			if it.Row != nil && s.ID == "needs-you" {
				red = append(red, it.Row.ID)
			}
		}
	}
	if len(red) > 0 {
		p.Actions = append(p.Actions, review(of.Machine, red))
	}
	if !of.Workspace {
		p.Actions = append(p.Actions, declarePublic(l))
	}
	if of.Workspace {
		p.Actions = append(p.Actions, sharePort(of.Machine))
	}
	for _, h := range l.Fleet.Hosts {
		for _, s := range h.Services {
			p.Actions = append(p.Actions, stopSharing(s, h.Name))
		}
	}
	return page.Seal(p)
}

func review(machine string, rows []string) page.Action {
	argv := []string{"ports"}
	if machine != "" {
		argv = append(argv, "--on", machine)
	}
	return page.Action{ID: ActionReview, Label: "Review", On: page.OnRow, Rows: rows, Priority: 1, Run: &page.Run{Terminal: true, Argv: argv}}
}

// value: the routes a port is declared public by, in the owner's words.
func routes() []page.Option {
	out := make([]page.Option, 0, len(catalog.Routes))
	for _, r := range catalog.Routes {
		out = append(out, page.Option{Value: r.Name, Label: r.Label})
	}
	return out
}

// invariant: Declare public… offers the ports that listen there and are
// not declared yet, red ones first.
func declarePublic(l ports.Listing) page.Action {
	var opts []page.Option
	for _, red := range []bool{true, false} {
		for _, h := range l.Fleet.Hosts {
			for _, p := range h.Ports {
				if p.Declared != nil || p.Red() != red || p.Reach == ports.ThisMachine {
					continue
				}
				opts = append(opts, page.Option{Value: "Machine=" + strconv.Quote(p.Machine.Name) + " Port=" + strconv.Quote(strconv.Itoa(p.Port)), Label: p.Machine.Name + ":" + strconv.Itoa(p.Port)})
			}
		}
	}
	form := &page.Form{Title: "Declare a port public", Fields: []page.Field{
		{Key: "Port", Label: "Port", Kind: page.FieldSelect, Options: opts, Required: true},
		{Key: "Via", Label: "Via", Kind: page.FieldSelect, Options: routes(), Default: catalog.RouteCloudflareTunnel, Required: true},
	}}
	if len(opts) > 0 {
		form.Fields[0].Default = opts[0].Value
	}
	return page.Action{ID: ActionDeclarePublic, Label: "Declare public", On: page.OnPage, Priority: 2, Mutates: true, Form: form, Run: &page.Run{Call: CallDeclare}}
}

func sharePort(vm string) page.Action {
	form := &page.Form{Title: "Share a port of " + vm, Fields: []page.Field{
		{Key: "Port", Label: "Port", Kind: page.FieldText, Required: true, Note: "the port it listens on, inside " + vm},
	}}
	return page.Action{ID: ActionSharePort, Label: "Share a port", On: page.OnPage, Priority: 2, Mutates: true, Form: form,
		Run: &page.Run{Call: CallShare, In: map[string]string{"VM": strconv.Quote(vm)}}}
}

func stopSharing(s ports.ServiceView, host string) page.Action {
	id := pageports.Row(s).ID
	port := strconv.Itoa(s.GuestPort)
	confirm := &page.Confirm{Question: "Stop sharing " + s.VM + "'s port " + port + "?",
		Lines: []page.Value{page.Text(released(s))},
		Yes:   "Stop sharing", No: "Cancel", Danger: true}
	return page.Action{ID: ActionStopSharing + id, Label: "Stop sharing", On: page.OnRow, Rows: []string{id}, Priority: 3, Mutates: true, Confirm: confirm,
		Run: &page.Run{Call: CallStopSharing, In: map[string]string{"VM": strconv.Quote(s.VM), "Port": strconv.Quote(port), "Host": strconv.Quote(host), "Confirm": "yes"}}}
}

func released(s ports.ServiceView) string {
	if s.HostPort == 0 {
		return "Its link stops working on " + catalog.TailnetProduct + "."
	}
	return "Its link stops working on " + catalog.TailnetProduct + ", and reserved port " + strconv.Itoa(s.HostPort) + " is let go."
}
