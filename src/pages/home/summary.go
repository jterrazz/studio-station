package home

import (
	"sort"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

// value: the Needs-you lines an Overview shows folded; --all shows every one.
const folded = 5

func Summary(s overview.Summary, here page.Context, all bool) page.Page {
	// invariant: SPEC §5.1: the first line sums the machine up, Needs you
	// comes first, then one line per section in the one order, then the VMs
	// of a Mac, then one or two next steps.
	sections := []page.Section{{ID: "overview", Title: title(s), Caption: caption(s), Joined: true, Role: page.Banner, Uncounted: true}}
	if len(s.Needs) > 0 {
		sections = append(sections, needsYou(s.Needs, all))
	}
	lines := make([]page.Item, 0, len(s.Lines)+1)
	for _, l := range s.Lines {
		row := page.Row{ID: string(l.Section), Label: sectionTitle(string(l.Section)), Cells: []page.Value{lineValue(l)}}
		if l.Pending {
			row.Mark = page.Awaiting
		}
		lines = append(lines, page.Item{Row: &row})
	}
	if s.Head.Here && len(s.VMs) > 0 {
		row := page.Row{ID: "vms", Label: "VMs", Cells: []page.Value{vmsValue(s.VMs)}}
		lines = append(lines, page.Item{Row: &row})
	}
	sections = append(sections, page.Section{ID: "sections", Role: page.Banner, Uncounted: true, Columns: []page.Column{
		{Key: "section", Kind: page.KindText, Keep: 100}, {Key: "line", Kind: page.KindList, Keep: 90},
	}, Items: lines})
	state := page.Ready
	if s.Pending {
		state = page.Loading
	}
	return page.Seal(page.Page{ID: "overview", Command: "overview", Context: here, State: state, Sections: sections, Hints: hints(s, all)})
}

func title(s overview.Summary) string {
	if s.Head.Here && s.Head.Kind == "mac" {
		return "This Mac"
	}
	return s.Head.Name
}

func caption(s overview.Summary) []page.Value {
	parts := []page.Value{}
	if title(s) != s.Head.Name {
		parts = append(parts, page.Text(s.Head.Name))
	}
	switch len(s.Needs) {
	case 0:
		parts = append(parts, page.Text("nothing needs you"))
	case 1:
		parts = append(parts, page.Text("1 thing needs you"))
	default:
		parts = append(parts, page.Text(strconv.Itoa(len(s.Needs))+" things need you"))
	}
	switch s.Engine {
	case overview.EngineRunning:
		parts = append(parts, page.Text("Engine running"))
	case overview.EngineStopped:
		parts = append(parts, page.Text("Engine stopped"))
	case overview.EngineNone:
	}
	return parts
}

func needsYou(needs []overview.Need, all bool) page.Section {
	items := make([]page.Item, 0, len(needs))
	for i, n := range needs {
		if !all && i == folded {
			items = append(items, page.Item{Note: &page.Note{Text: strconv.Itoa(len(needs)-folded) + " more: station overview --all"}})
			break
		}
		mark, reason := page.Attention, page.Degraded
		if n.Broken {
			mark, reason = page.Broken, page.Failing
		}
		row := page.Row{ID: strconv.Itoa(i) + "-" + n.Subject, Label: n.Text, Mark: mark, Reason: reason, Cells: []page.Value{fixValue(n)}}
		items = append(items, page.Item{Row: &row})
	}
	columns := []page.Column{{Key: "finding", Kind: page.KindText, Keep: 100}, {Key: "fix", Kind: page.KindList, Keep: 50}}
	return page.Section{ID: "needs-you", Title: "Needs you", Uncounted: true, Columns: columns, Items: items}
}

func fixValue(n overview.Need) page.Value {
	switch {
	case n.Call != "":
		return page.Text(fixWords(n.Call, n.Args))
	case n.Command != "":
		return page.Text(n.Command)
	case n.Path != "":
		return page.Text(n.Path)
	}
	return page.None()
}

func fixWords(name string, args map[string]string) string {
	// invariant: a fix's call printed as the command that runs it: its noun
	// and verb, the name or path it acts on, then each other input a flag.
	noun, verb, _ := strings.Cut(name, ".")
	words := []string{"station", noun}
	if verb != "list" && verb != "read" && verb != "run" {
		words = append(words, verb)
	}
	for _, key := range []string{"name", "path"} {
		if v, ok := args[key]; ok {
			words = append(words, v)
		}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		if k != "name" && k != "path" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		words = append(words, "--"+k, args[k])
	}
	return strings.Join(words, " ")
}

func lineValue(l overview.Line) page.Value {
	switch {
	case l.Pending:
		return page.Pending()
	case l.Unread != "":
		return page.Text(l.Unread)
	}
	parts := make([]page.Value, 0, len(l.Parts))
	for _, p := range l.Parts {
		parts = append(parts, partValue(p))
	}
	return page.List(parts...)
}

func partValue(p overview.Part) page.Value {
	var v page.Value
	switch {
	case p.Bytes > 0 || p.Word == "free" || p.Word == "can rebuild":
		v = page.Size(p.Bytes)
		v.Unit = p.Word
	case p.Of > 0:
		v = page.Text(strconv.Itoa(p.N) + " of " + strconv.Itoa(p.Of))
	case p.Counted && p.State == "":
		v = page.Text(p.Word + " " + strconv.Itoa(p.N))
	case p.Counted && p.Word == "":
		v = page.Text(strconv.Itoa(p.N))
	case p.Counted:
		v = page.Count(int64(p.N), p.Word)
	default:
		v = page.Text(p.Word)
	}
	if p.State == "" {
		return v
	}
	if p.State == catalog.StateUnknown {
		return page.Labeled(string(page.NotKnown), v)
	}
	return page.Labeled(string(pages.MarkOf(p.State)), v)
}

func vmsValue(vms []overview.VM) page.Value {
	parts := make([]page.Value, 0, len(vms))
	for _, vm := range vms {
		if vm.Running {
			parts = append(parts, page.Labeled(string(page.OK), page.Text(vm.Name+" running")))
			continue
		}
		parts = append(parts, page.Labeled(string(page.Absent), page.Text(vm.Name+" stopped")))
	}
	return page.List(parts...)
}

func hints(s overview.Summary, all bool) []page.Hint {
	var out []page.Hint
	for _, n := range s.Needs {
		if n.Call != "" {
			out = append(out, page.Hint{Command: fixWords(n.Call, n.Args)})
			break
		}
	}
	if !all && len(s.Needs) > folded {
		out = append(out, page.Hint{Command: "station overview --all", Text: "for every line"})
	}
	return out
}

func sectionTitle(id string) string {
	if id == "" {
		return ""
	}
	return strings.ToUpper(id[:1]) + id[1:]
}
