package fleet

import (
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/page"
	pageimages "github.com/jterrazz/studio-station/src/pages/images"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	pagesecrets "github.com/jterrazz/studio-station/src/pages/secrets"
	pageservices "github.com/jterrazz/studio-station/src/pages/services"
	pagetailnet "github.com/jterrazz/studio-station/src/pages/tailnet"
)

const (
	KeyBuild  = "b"
	KeyForget = "f"
	KeyProbe  = "p"
	KeyUnlock = "o"
)

type Reading struct {
	Machines *machines.Fleet
	Images   *[]images.Report
	Secrets  *secrets.Report
	Services *services.Fleet
	Tailnet  *tailnet.Status
	Failed   map[string]page.Error
}

func Tab(r Reading, here page.Context) page.Page {
	p := page.Page{ID: "fleet", Context: here}
	p.Sections = append(p.Sections, part(r, "fleet", "Fleet", r.Machines != nil, func() []page.Section {
		s := pagemachines.Section(*r.Machines, links(r.Services))
		s.Fold = page.Open
		return []page.Section{s}
	})...)
	p.Sections = append(p.Sections, part(r, "images", "Images", r.Images != nil, func() []page.Section {
		s := pageimages.Section(*r.Images)
		s.Role, s.Fold = page.Plain, page.Open
		return []page.Section{s}
	})...)
	p.Sections = append(p.Sections, part(r, "secrets", "Secrets", r.Secrets != nil, func() []page.Section {
		s := nest(page.Section{ID: "secrets", Title: "Secrets", Uncounted: true}, pagesecrets.Sections(*r.Secrets))
		return []page.Section{s}
	})...)
	p.Sections = append(p.Sections, part(r, "tailnet", "Tailnet", r.Tailnet != nil, func() []page.Section {
		parts := pagetailnet.Sections(*r.Tailnet)
		head := parts[0]
		head.Role = page.Plain
		return []page.Section{nest(head, parts[1:])}
	})...)
	p.Actions = Actions(r)
	return page.Seal(p)
}

// invariant: on the Fleet tab a page's sub-lists (Secrets' Lend list and
// Kept, A19; Tailnet's Peers and Serve) are heading groups of ONE section, so
// folding the section folds them; the commands keep their titled sections.
func nest(s page.Section, parts []page.Section) page.Section {
	s.Fold = page.Auto
	if len(parts) > 0 && len(s.Columns) < len(parts[0].Columns) {
		s.Columns = parts[0].Columns
	}
	if len(s.Columns) == 0 && len(parts) > 0 {
		s.Columns = parts[0].Columns
	}
	for _, sub := range parts {
		items := make([]page.Item, 0, len(sub.Items))
		for _, it := range sub.Items {
			if it.Row != nil {
				row := *it.Row
				row.ID = sub.ID + "/" + row.ID
				it.Row = &row
			}
			items = append(items, it)
		}
		s.Items = append(s.Items, page.Item{Group: &page.Group{Head: page.Row{ID: sub.ID, Label: sub.Title}, Items: items, Fold: page.Open}})
	}
	return s
}

func part(r Reading, id, title string, ready bool, build func() []page.Section) []page.Section {
	if e, failed := r.Failed[id]; failed {
		return []page.Section{{ID: id, Title: title, Columns: []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}}, Items: []page.Item{{Error: &e}}}}
	}
	if !ready {
		return []page.Section{{ID: id, Title: title, Columns: []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}},
			Items: []page.Item{{Row: &page.Row{ID: id, Label: "reading", Mark: page.Awaiting}}}}}
	}
	return build()
}

func links(f *services.Fleet) func(host, vm string) []page.Link {
	if f == nil {
		return nil
	}
	return func(host, vm string) []page.Link {
		var out []page.Link
		for _, h := range f.Hosts {
			if h.Name != host {
				continue
			}
			for _, s := range h.Services {
				if s.VM == vm {
					out = append(out, pageservices.Link(s))
				}
			}
		}
		return out
	}
}

func Actions(r Reading) []page.Action {
	var vms, lasting, remote, hosts []string
	if r.Machines != nil {
		for _, h := range r.Machines.Hosts {
			hosts = append(hosts, h.Name)
			if !h.Here {
				remote = append(remote, h.Name)
			}
			for _, vm := range h.VMs {
				vms = append(vms, vm.Name)
				remote = append(remote, vm.Name)
				if vm.Lifetime == machines.Lasting {
					lasting = append(lasting, vm.Name)
				}
			}
		}
	}
	var out []page.Action
	add := func(a page.Action, rows []string) {
		if a.On == page.OnPage || len(rows) > 0 {
			a.Rows = rows
			out = append(out, a)
		}
	}
	add(page.Action{Key: pagemachines.KeyDelete, Label: "delete", On: page.OnRow, Priority: 1, Mutates: true, Confirm: &page.Confirm{Yes: "delete", Danger: true}}, vms)
	add(page.Action{Key: pagemachines.KeyRestart, Label: "restart", On: page.OnRow, Priority: 2, Mutates: true, Confirm: &page.Confirm{Yes: "restart", Danger: true}}, append(append([]string{}, hosts...), vms...))
	add(page.Action{Key: pagemachines.KeyEnter, Label: "enter", On: page.OnRow, Priority: 3, Mutates: true}, remote)
	add(page.Action{Key: pagemachines.KeyCreate, Label: "create", On: page.OnPage, Priority: 4, Mutates: true}, nil)
	add(page.Action{Key: pagemachines.KeyStart, Label: "start/stop", On: page.OnRow, Priority: 5, Mutates: true}, lasting)
	add(page.Action{Key: KeyForget, Label: "forget", On: page.OnRow, Priority: 6, Mutates: true, Confirm: &page.Confirm{Yes: "forget"}}, without(remote, vms))
	add(page.Action{Key: KeyBuild, Label: "build images", On: page.OnPage, Priority: 7, Mutates: true}, nil)
	add(page.Action{Key: KeyProbe, Label: "probe", On: page.OnRow, Priority: 8}, remote)
	add(page.Action{Key: KeyUnlock, Label: "unlock", On: page.OnRow, Priority: 9, Mutates: true}, without(remote, vms))
	return out
}

func without(list, drop []string) []string {
	var out []string
	for _, x := range list {
		keep := true
		for _, d := range drop {
			keep = keep && x != d
		}
		if keep {
			out = append(out, x)
		}
	}
	return out
}

func DeleteConfirm(name string) page.Confirm {
	return page.Confirm{Question: "Delete " + name + "?", Lines: []page.Value{page.Text("everything it holds is destroyed")}, Danger: true, Yes: "delete"}
}

func ForgetConfirm(name string) page.Confirm {
	return page.Confirm{Question: "Forget " + name + "?", Lines: []page.Value{page.Text("the host itself is not touched")}, Yes: "forget"}
}

func RestartConfirm(name string, host bool) page.Confirm {
	c := page.Confirm{Question: "Restart " + name + "?", Danger: true, Yes: "restart"}
	if host {
		c.Type = name
	}
	return c
}

func CreateForm(hosts []pagemachines.HostChoice, imageNames []string) page.Form {
	f := pagemachines.CreateForm("", hosts)
	f.Title = "Create a VM"
	image := page.Field{Key: "image", Label: "Image", Kind: page.FieldSelect, Required: true}
	for _, n := range imageNames {
		image.Options = append(image.Options, page.Option{Value: n})
	}
	f.Fields = append(f.Fields, image, page.Field{Key: "name", Label: "Name", Kind: page.FieldText, Note: "empty: <image>-<n>"})
	return f
}
