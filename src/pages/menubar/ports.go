package menubar

import (
	"slices"
	"strconv"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/page"
	pageports "github.com/jterrazz/studio-station/src/pages/ports"
	"github.com/jterrazz/studio-station/src/ports"
)

const (
	PortsID page.ID = "menubar.ports"
	VMID    page.ID = "menubar.vm"

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
}

func Ports(l ports.Listing, of PortsOf, here page.Context) page.Page {
	// invariant: SPEC §10.2 Ports and A port: the ports page of §5.6, red on
	// top, then Public, then Private, each row's class in words; every action
	// it shows is a call, but Review, which opens a terminal on the same list.
	return portsPage(PortsID, l, of, here, declarePublic(l))
}

func Workspace(l ports.Listing, vm string, here page.Context) page.Page {
	// invariant: SPEC §10.2 A workspace, PO-45: a workspace's page lists its
	// own ports alone, each shared one with its link, and offers Share a
	// port… of it, never Declare public.
	return portsPage(VMID, only(l, vm), PortsOf{Machine: vm}, here, sharePort(vm))
}

func portsPage(id page.ID, l ports.Listing, of PortsOf, here page.Context, offer page.Action) page.Page {
	p := pageports.Listed(l, here, false)
	p.ID = id
	p.Actions = nil
	if of.Machine != "" && len(p.Sections) > 0 && p.Sections[0].ID == "summary" {
		p.Sections[0].Items[0].Row.Label = of.Machine
	}
	p.Actions = append(p.Actions, reviews(p, l, of)...)
	p.Actions = append(p.Actions, offer)
	for _, h := range l.Fleet.Hosts {
		for _, s := range h.Services {
			p.Actions = append(p.Actions, stopSharing(s, h.Name))
		}
	}
	return page.Seal(p)
}

// value: the listing of one VM: its listeners and its shares, on the Mac
// it runs on.
func only(l ports.Listing, vm string) ports.Listing {
	out := l
	out.Fleet.Hosts = nil
	for _, h := range l.Fleet.Hosts {
		h.Services = slices.DeleteFunc(slices.Clone(h.Services), func(s ports.ServiceView) bool { return s.VM != vm })
		h.Ports = slices.DeleteFunc(slices.Clone(h.Ports), func(p ports.Port) bool { return p.Machine.Name != vm })
		h.Listening = slices.DeleteFunc(slices.Clone(h.Listening), func(m ports.Machine) bool { return m.Ref.Name != vm })
		if len(h.Services) > 0 || len(h.Ports) > 0 {
			out.Fleet.Hosts = append(out.Fleet.Hosts, h)
		}
	}
	return out
}

// invariant: Review opens a terminal on the ports of the Mac its red rows
// are on: the page's Mac, or each Mac of a page of every Mac, This Mac's
// without --on.
func reviews(p page.Page, l ports.Listing, of PortsOf) []page.Action {
	red := map[string]bool{}
	for _, s := range p.Sections {
		for _, it := range s.Items {
			if it.Row != nil && s.ID == "needs-you" {
				red[it.Row.ID] = true
			}
		}
	}
	var out []page.Action
	var hosts []string
	for _, h := range l.Fleet.Hosts {
		var rows []string
		for _, port := range h.Ports {
			if red[port.Key()] {
				rows = append(rows, port.Key())
			}
		}
		if len(rows) == 0 {
			continue
		}
		machine := of.Machine
		if machine == "" && !h.Here {
			machine = h.Name
		}
		if of.Machine != "" && len(out) > 0 {
			out[0].Rows = append(out[0].Rows, rows...)
			continue
		}
		out = append(out, review(machine, rows))
		hosts = append(hosts, h.Name)
	}
	if len(out) > 1 {
		for i := range out {
			out[i].ID = ActionReview + ":" + hosts[i]
		}
	}
	return out
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
