package tui

import (
	"strings"

	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/page"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type node struct {
	name   string
	label  string
	kind   machine.Kind
	mark   page.Mark
	mac    string
	here   bool
	engine string
	facts  []string
}

const allMachines = "All machines"

func (n node) all() bool { return n.kind == "" }

func (n node) heading() string {
	switch n.kind {
	case "":
		return "ALL MACHINES"
	case machine.Mac:
		return strings.ToUpper(n.label)
	default:
		return strings.ToUpper(n.label) + ui.Sep + strings.ToUpper(string(n.kind))
	}
}

func tree(here page.Context, p *page.Page) []node {
	nodes := []node{{label: allMachines}}
	this := node{name: here.Machine, label: "This Mac", kind: machine.Mac, mark: page.Awaiting, here: true}
	if p == nil {
		return append(nodes, this)
	}
	prefix, _, _ := strings.Cut(here.Machine, "-")
	for _, s := range p.Sections {
		for _, it := range s.Items {
			if it.Group == nil {
				continue
			}
			h := it.Group.Head
			mac := node{name: h.ID, label: h.Label, kind: machine.Mac, mark: h.Mark}
			if h.ID == here.Machine {
				mac.label, mac.here = "This Mac", true
			} else if short, ok := strings.CutPrefix(h.Label, prefix+"-"); ok && short != "" {
				mac.label = short
			}
			if len(h.Cells) > 2 {
				mac.engine = ui.Plain(h.Cells[2])
			}
			mac.facts = []string{h.ID}
			nodes = append(nodes, mac)
			var servers, workspaces []node
			for _, v := range it.Group.Items {
				if v.Row == nil {
					continue
				}
				r := *v.Row
				vm := node{name: r.ID, label: r.Label, mark: r.Mark, mac: mac.name}
				if len(r.Cells) > 0 {
					vm.kind = machine.Kind(strings.ToLower(ui.Plain(r.Cells[0])))
					vm.facts = append(vm.facts, ui.Plain(r.Cells[0]))
				}
				if len(r.Cells) > 2 {
					for _, w := range r.Cells[2].Items {
						word := ui.Plain(w)
						vm.facts = append(vm.facts, strings.ToUpper(word[:1])+word[1:])
					}
				}
				vm.facts = append(vm.facts, "on "+mac.label)
				if vm.kind == machine.Server {
					servers = append(servers, vm)
				} else {
					workspaces = append(workspaces, vm)
				}
			}
			nodes = append(append(nodes, servers...), workspaces...)
		}
	}
	return nodes
}

func find(nodes []node, name string) (node, bool) {
	for _, n := range nodes {
		if n.name == name {
			return n, true
		}
	}
	return node{}, false
}

func words(n node, s section.ID) []string {
	if n.all() {
		switch s {
		case section.Overview:
			return []string{"machines"}
		case section.Images:
			return []string{"images"}
		default:
			return []string{string(s), "--all-machines"}
		}
	}
	var w []string
	switch s {
	case section.Overview:
		w = []string{"overview", "--all"}
	case section.SSH:
		return []string{"machines", string(s), n.name}
	default:
		w = []string{string(s)}
	}
	if !n.here {
		w = append(w, "--on", n.name)
	}
	return w
}
