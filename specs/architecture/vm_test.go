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
)

// invariant: rule one-way-into-a-vm, ARCHITECTURE 13.3 rule 11: a VM is
// reached through its engine's one Guest connection alone. No code runs ssh
// but the two adapters that reach a Mac, no code runs scp at all, and only the
// engine (its dialer handed by wire's engineGuests) and the contract's own
// adapters and suite dial a Guest.
func sshExecOwners() []string { return []string{"remote/adapter/sshstdio", "machines/adapter/macos"} }

// invariant: the allowlist of one-way-into-a-vm's ssh execs: each file that
// still runs ssh, with the lot that removes it; each reaches a Mac. It only
// shrinks; a line no longer true fails the gate until removed.
func sshExecTolerated() map[string]string {
	return map[string]string{
		"src/domain/machines/probe.go": "L20: a Mac's probe, over its engine, once probe is an alias of the machines calls",
	}
}

// invariant: the allowlist of one-way-into-a-vm's Guest dials outside the
// engine, each with why it is not yet the engine's. It only shrinks.
func guestDialTolerated() map[string]string {
	return map[string]string{
		"src/domain/images/build.go": "a build VM is none of the engine's until images.build is a call it runs",
	}
}

const guestDialer = module + "/src/machines/contract.GuestDialer"

func dialsAGuest(f *file, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Dial" {
		return false
	}
	t := f.info.TypeOf(sel.X)
	if t == nil {
		return false
	}
	return types.TypeString(t, nil) == guestDialer
}

func mayDialAGuest(f *file) bool {
	switch {
	case f.pkg == "engine" || strings.HasPrefix(f.pkg, "engine/"):
		return true
	case f.rel == "src/wire/guest_dialer.go":
		return true
	case strings.HasPrefix(f.pkg, "machines/adapter/") || f.pkg == "machines/contract/contracttest":
		return true
	}
	return false
}

// invariant: a literal runs a tool when it names it as a command's Name, as
// an argv's first word (a slice literal's, or the first an append adds), or
// as exec.Command's first argument.
func execNames(f *file, visit func(at, tool string)) {
	tool := func(e ast.Expr) (string, bool) {
		lit, ok := e.(*ast.BasicLit)
		if !ok {
			return "", false
		}
		v, err := strconv.Unquote(lit.Value)
		return v, err == nil && (v == "ssh" || v == "scp")
	}
	ast.Inspect(f.syntax, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.KeyValueExpr:
			if k, ok := x.Key.(*ast.Ident); ok && k.Name == "Name" {
				if v, ok := tool(x.Value); ok {
					visit(f.at(x.Pos()), v)
				}
			}
		case *ast.CompositeLit:
			if len(x.Elts) > 0 {
				if v, ok := tool(x.Elts[0]); ok {
					visit(f.at(x.Pos()), v)
				}
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Command" && len(x.Args) > 0 {
				if v, ok := tool(x.Args[0]); ok {
					visit(f.at(x.Pos()), v)
				}
			}
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "append" && len(x.Args) > 1 {
				if v, ok := tool(x.Args[1]); ok {
					visit(f.at(x.Pos()), v)
				}
			}
		}
		return true
	})
}

func ruleOneWayIntoAVM(w *world) []finding {
	var out []finding
	execs, dials := map[string]bool{}, map[string]bool{}
	for _, f := range w.files {
		owner := slices.Contains(sshExecOwners(), f.pkg)
		execNames(f, func(at, tool string) {
			switch {
			case tool == "scp":
				out = append(out, finding{"one-way-into-a-vm", at, "scp runs nowhere: a file reaches a VM by Guest.Put"})
			case owner:
			case sshExecTolerated()[f.rel] != "":
				execs[f.rel] = true
			default:
				out = append(out, finding{"one-way-into-a-vm", at, fmt.Sprintf("%q runs ssh: a VM is reached by its engine's Guest, a Mac by remote/adapter/sshstdio", f.pkg)})
			}
		})
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !dialsAGuest(f, call) || mayDialAGuest(f) {
				return true
			}
			if guestDialTolerated()[f.rel] != "" {
				dials[f.rel] = true
				return true
			}
			out = append(out, finding{"one-way-into-a-vm", f.at(call.Pos()), fmt.Sprintf("%q dials a Guest: only the engine holds a VM's connection", f.pkg)})
			return true
		})
	}
	for _, rel := range slices.Sorted(maps.Keys(sshExecTolerated())) {
		if !execs[rel] {
			out = append(out, finding{"one-way-into-a-vm", "specs/architecture/vm_test.go", rel + " no longer runs ssh: remove it from sshExecTolerated"})
		}
	}
	for _, rel := range slices.Sorted(maps.Keys(guestDialTolerated())) {
		if !dials[rel] {
			out = append(out, finding{"one-way-into-a-vm", "specs/architecture/vm_test.go", rel + " no longer dials a Guest: remove it from guestDialTolerated"})
		}
	}
	return out
}

func TestOneWayIntoAVM(t *testing.T)     { judge(t, "OneWayIntoAVM") }
func TestOneWayIntoAVMSCP(t *testing.T)  { judge(t, "OneWayIntoAVMSCP") }
func TestOneWayIntoAVMDial(t *testing.T) { judge(t, "OneWayIntoAVMDial") }

func TestOneWayIntoAVMAllowlistShrinks(t *testing.T) {
	clean, _ := worlds(t)
	rel := slices.Sorted(maps.Keys(sshExecTolerated()))[0]
	gone := &world{root: clean.root, packages: clean.packages, cgo: clean.cgo}
	for _, f := range clean.files {
		if f.rel != rel {
			gone.files = append(gone.files, f)
		}
	}
	if !slices.ContainsFunc(ruleOneWayIntoAVM(gone), func(f finding) bool { return strings.HasPrefix(f.what, rel+" no longer runs ssh") }) {
		t.Errorf("the one-way-into-a-vm rule kept the line of %s, a file that is gone", rel)
	}
}
