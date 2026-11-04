package architecture_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	accountscalls "github.com/jterrazz/studio-station/src/accounts/calls"
	appscalls "github.com/jterrazz/studio-station/src/apps/calls"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
	diskcalls "github.com/jterrazz/studio-station/src/disk/calls"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	setupcalls "github.com/jterrazz/studio-station/src/setup/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/words"
)

// invariant: the allowlist of calls-gated: the words a family's calls are
// still named by before its v3 words land (tools.install in apps). It only
// shrinks; a word no call of its family takes fails the gate.
func callWordsTolerated() map[string][]string {
	return map[string][]string{
		"apps":     {"tools"},
		"settings": {"config", "run"},
		"disk":     {"storage"},
		"awake":    {"power"},
		"accounts": {"secrets"},
		"ports":    {"services"},
	}
}

type callValue struct {
	f      *file
	name   string
	lit    *ast.CompositeLit
	fields map[string]ast.Expr
}

// value: every package-level Call or Watch value a family's calls package
// declares, with its literal's fields.
func callValues(w *world) []callValue {
	var out []callValue
	for _, f := range w.files {
		if f.test || !isCalls(f.pkg) {
			continue
		}
		for _, d := range f.syntax.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range g.Specs {
				v, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range v.Names {
					if i >= len(v.Values) || !isCallType(f.info.TypeOf(v.Values[i])) {
						continue
					}
					lit, ok := v.Values[i].(*ast.CompositeLit)
					if !ok {
						continue
					}
					fields := map[string]ast.Expr{}
					for _, e := range lit.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok {
								fields[k.Name] = kv.Value
							}
						}
					}
					out = append(out, callValue{f: f, name: n.Name, lit: lit, fields: fields})
				}
			}
		}
	}
	return out
}

func isCallType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == module+"/src/core/call" && (named.Obj().Name() == "Call" || named.Obj().Name() == "Watch")
}

// value: the identifiers a package's All function body names.
func registered(w *world, pkg string) map[string]bool {
	out := map[string]bool{}
	for _, f := range w.files {
		if f.pkg != pkg || f.test {
			continue
		}
		for _, d := range f.syntax.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "All" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					out[id.Name] = true
				}
				return true
			})
		}
	}
	return out
}

// invariant: rule calls-gated (AR-12): every exported Call or Watch of a
// family's calls package is in its All, names itself <family>.<verb> once
// (today's words allowlisted), and sets its Applies.
func ruleCallsGated(w *world) []finding {
	var out []finding
	names := map[string]string{}
	taken := map[string]bool{}
	for _, c := range callValues(w) {
		if !ast.IsExported(c.name) {
			continue
		}
		at := c.f.at(c.lit.Pos())
		family := topOf(c.f.pkg)
		if !registered(w, c.f.pkg)[c.name] {
			out = append(out, finding{"calls-gated", at, c.name + " is not in " + c.f.pkg + ".All"})
		}
		if _, ok := c.fields["Applies"]; !ok {
			out = append(out, finding{"calls-gated", at, c.name + " sets no Applies"})
		}
		lit, ok := c.fields["Name"].(*ast.BasicLit)
		if !ok {
			out = append(out, finding{"calls-gated", at, c.name + " names itself by no literal"})
			continue
		}
		name, _ := strconv.Unquote(lit.Value)
		word, verb, dotted := strings.Cut(name, ".")
		switch {
		case !dotted || verb == "" || strings.Contains(verb, "."):
			out = append(out, finding{"calls-gated", at, fmt.Sprintf("%q is not <family>.<verb>", name)})
		case word != family && !slices.Contains(callWordsTolerated()[family], word):
			out = append(out, finding{"calls-gated", at, fmt.Sprintf("%q is not named by its family %s", name, family)})
		}
		taken[family+"/"+word] = true
		if other, used := names[name]; used {
			out = append(out, finding{"calls-gated", at, fmt.Sprintf("%q is already %s", name, other)})
		}
		names[name] = at
	}
	for _, family := range slices.Sorted(maps.Keys(callWordsTolerated())) {
		for _, word := range callWordsTolerated()[family] {
			if !taken[family+"/"+word] {
				out = append(out, finding{"calls-gated", "specs/architecture/calls_test.go", "no call of " + family + " is named " + word + ".*: remove it from callWordsTolerated"})
			}
		}
	}
	return out
}

func requires(fn ast.Node, info *types.Info) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
			if obj := info.Uses[sel.Sel]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == module+"/src/core/confirm" && obj.Name() == "Require" {
				found = true
			}
		}
		return !found
	})
	return found
}

// invariant: rule confirm-in-usecase (SC-07): a call that names a
// confirmation asks it in its own Run; no surface asks the rule itself.
func ruleConfirmInUsecase(w *world) []finding {
	var out []finding
	for _, c := range callValues(w) {
		if _, asks := c.fields["Confirm"]; asks && !requires(c.fields["Run"], c.f.info) {
			out = append(out, finding{"confirm-in-usecase", c.f.at(c.lit.Pos()), c.name + " names a confirmation its Run never requires"})
		}
	}
	for _, f := range w.files {
		if f.test || topOf(f.pkg) != "surface" {
			continue
		}
		if requires(f.syntax, f.info) {
			out = append(out, finding{"confirm-in-usecase", f.rel, "a surface decides a confirmation (confirm.Require): only a use case does"})
		}
	}
	return out
}

// invariant: rule every-code-worded (ER-07): every fail.Code constant the
// tree declares is worded in surface/cli/words, and nothing worded is
// never raised.
func ruleEveryCodeWorded(w *world) []finding {
	var out []finding
	worded := map[string]bool{}
	for code := range words.Codes() {
		worded[string(code)] = true
	}
	declared := map[string]bool{}
	for _, f := range w.files {
		if f.test {
			continue
		}
		for _, d := range f.syntax.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range g.Specs {
				v, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range v.Names {
					c, ok := f.info.Defs[n].(*types.Const)
					if !ok || !isCode(c.Type()) {
						continue
					}
					value := c.Val().ExactString()
					code, _ := strconv.Unquote(value)
					declared[code] = true
					if !worded[code] {
						out = append(out, finding{"every-code-worded", f.at(n.Pos()), fmt.Sprintf("%q is worded nowhere in surface/cli/words", code)})
					}
				}
			}
		}
	}
	for _, code := range slices.Sorted(maps.Keys(worded)) {
		if !declared[code] {
			out = append(out, finding{"every-code-worded", "src/surface/cli/words/codes.go", fmt.Sprintf("%q is worded but no package declares it", code)})
		}
	}
	return out
}

func isCode(t types.Type) bool {
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == module+"/src/core/fail" && named.Obj().Name() == "Code"
}

func TestCallsGated(t *testing.T)       { judge(t, "CallsGated") }
func TestConfirmInUsecase(t *testing.T) { judge(t, "ConfirmInUsecase") }
func TestEveryCodeWorded(t *testing.T)  { judge(t, "EveryCodeWorded") }

// value: the reference profiles of the worlds: a laptop Mac, a desktop
// Mac, a workspace, a server.
func referenceProfiles() []machine.Profile {
	return []machine.Profile{
		{Ref: machine.Ref{Name: "laptop"}, Kind: machine.Mac, System: machine.System{OS: machine.MacOS}, Caps: machine.Capabilities{Battery: true, Lid: true, Jobs: true, VMs: true, Engine: true}},
		{Ref: machine.Ref{Name: "desktop"}, Kind: machine.Mac, System: machine.System{OS: machine.MacOS}, Caps: machine.Capabilities{Jobs: true, VMs: true, Engine: true}},
		{Ref: machine.Ref{Name: "workspace", On: "laptop"}, Kind: machine.Workspace, System: machine.System{OS: machine.Linux}},
		{Ref: machine.Ref{Name: "server", On: "desktop"}, Kind: machine.Server, System: machine.System{OS: machine.Linux}},
	}
}

func TestEveryCallIsRegisteredOnceAndShowsItsSectionWhereverItApplies(t *testing.T) {
	registry, err := call.NewRegistry(
		accountscalls.All(accountscalls.Deps{}), appscalls.All(appscalls.Deps{}), awakecalls.All(awakecalls.Deps{}),
		diskcalls.All(diskcalls.Deps{}), imagescalls.All(imagescalls.Deps{}), machinescalls.All(machinescalls.Deps{}),
		portscalls.All(portscalls.Deps{}), settingscalls.All(settingscalls.Deps{}), setupcalls.All(setupcalls.Deps{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range registry.Metas() {
		if m.Applies == nil {
			t.Errorf("%s sets no Applies", m.Name)
			continue
		}
		for _, p := range referenceProfiles() {
			if m.Section != "" && m.Applies(p).OK && !slices.Contains(section.For(p.Kind), m.Section) {
				t.Errorf("%s lets a %s through, which has no %s section", m.Name, p.Kind, m.Section)
			}
		}
	}
}
