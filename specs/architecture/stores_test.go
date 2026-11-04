package architecture_test

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"maps"
	"slices"
	"strings"
	"testing"
)

// invariant: rule one-writer-per-store, ARCHITECTURE 12 and 13.3 rule 7: each
// store and each preference group has one owner, the calls package of its
// family; only it writes the store (Put, Delete, Set, Write), only wire
// builds a store's adapter, and each group is written by one contract.
func storeOwners() map[string]string {
	src := module + "/src/"
	return map[string]string{
		src + "core/contract.Preferences":           "core/contract",
		src + "setup/contract.NamePrefs":            "setup/calls",
		src + "setup/contract.IdentityPrefs":        "setup/calls",
		src + "machines/contract.HostsPrefs":        "machines/calls",
		src + "machines/contract.VMUserPrefs":       "machines/calls",
		src + "accounts/contract.ShareListPrefs":    "accounts/calls",
		src + "ports/contract.ServicesPrefs":        "ports/calls",
		src + "machines/contract.MachineStore":      "machines/calls",
		src + "images/contract.ImageStore":          "images/calls",
		src + "accounts/contract.ShareStore":        "accounts/calls",
		src + "ports/contract.ReservationStore":     "ports/calls",
		src + "ports/contract.DeclarationStore":     "ports/calls",
		src + "settings/contract.AppliedStore":      "settings/calls",
		src + "engine.Reservations":                 "ports/calls",
		src + "domain/services.Reservations":        "ports/calls",
		src + "machines/adapter/jsonfile.Store":     "machines/calls",
		src + "images/adapter/jsonfile.Store":       "images/calls",
		src + "accounts/adapter/jsonfile.Shares":    "accounts/calls",
		src + "ports/adapter/jsonfile.Reservations": "ports/calls",
		src + "ports/adapter/jsonfile.Declarations": "ports/calls",
		src + "settings/adapter/jsonfile.Applied":   "settings/calls",
		src + "core/adapter/jsonfile.Preferences":   "core/contract",
	}
}

// invariant: the allowlist of one-writer-per-store: each file of today's
// tree that still writes a store its family's calls will own, by the store
// it writes, with the lot that moves it. It only shrinks; a line no longer
// true fails the gate until it is removed.
func storeWritersTolerated() map[string]string {
	src := module + "/src/"
	return map[string]string{
		"src/domain/machines/hosts.go " + src + "setup/contract.NamePrefs":              "L19: the name is setup's, written by setup.run",
		"src/domain/machines/hosts.go " + src + "machines/contract.HostsPrefs":          "L14: machines.add and machines.remove become calls",
		"src/domain/machines/hosts.go " + src + "machines/contract.VMUserPrefs":         "L14: setup.run hands the VM user to machines' call",
		"src/domain/machines/create.go " + src + "machines/contract.MachineStore":       "L14: machines.create and machines.delete become calls",
		"src/domain/images/records.go " + src + "images/contract.ImageStore":            "L14: images.build becomes a call",
		"src/domain/images/service.go " + src + "images/contract.ImageStore":            "L14: images.delete becomes a call",
		"src/domain/secrets/service.go " + src + "accounts/contract.ShareListPrefs":     "L14: accounts.allow and accounts.disallow become calls",
		"src/domain/secrets/service.go " + src + "accounts/contract.ShareStore":         "L14: accounts.share and accounts.take-back become calls",
		"src/domain/config/claude_token.go " + src + "accounts/contract.ShareListPrefs": "L25: the claude-token setting runs accounts.allow",
		"src/domain/services/lease.go " + src + "domain/services.Reservations":          "L14: ports.share and ports.stop-sharing become calls the engine runs",
		"src/domain/services/service.go " + src + "domain/services.Reservations":        "L14: ports.stop-sharing becomes a call",
		"src/domain/setup/layout.go " + src + "setup/contract.NamePrefs":                "L19: the v1 conversion becomes the Preferences migration",
		"src/domain/setup/layout.go " + src + "setup/contract.IdentityPrefs":            "L19: the v1 conversion becomes the Preferences migration",
		"src/domain/setup/layout.go " + src + "machines/contract.HostsPrefs":            "L19: the v1 conversion becomes the Preferences migration",
		"src/domain/setup/layout.go " + src + "machines/contract.VMUserPrefs":           "L19: the v1 conversion becomes the Preferences migration",
	}
}

func storeWrite(name string) bool {
	return slices.Contains([]string{"Put", "Delete", "Set", "Write"}, name)
}

func storeOf(f *file, sel *ast.SelectorExpr) (string, bool) {
	t := f.info.TypeOf(sel.X)
	if t == nil {
		return "", false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	name := types.TypeString(t, nil)
	_, ok := storeOwners()[name]
	return name, ok
}

// invariant: a store's own packages write it freely: its contract (the typed
// groups over Preferences), its adapters and its suite.
func ownStorePackage(pkg, store string) bool {
	owner := strings.TrimPrefix(store, module+"/src/")
	family := topOf(owner)
	return pkg == family+"/contract" || pkg == family+"/contract/contracttest" || strings.HasPrefix(pkg, family+"/adapter/")
}

// invariant: Preferences is written raw only by the typed groups, each in its
// owner's contract package.
func mayWrite(pkg, store string) bool {
	if store == preferences {
		return restOf(pkg) == "contract" || ownStorePackage(pkg, store)
	}
	return pkg == storeOwners()[store] || ownStorePackage(pkg, store)
}

const preferences = module + "/src/core/contract.Preferences"

func ruleOneWriterPerStore(w *world) []finding {
	var out []finding
	seen := map[string]bool{}
	groups := map[string]string{}
	for _, f := range w.files {
		if f.test {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if obj, isFunc := f.info.Uses[sel.Sel].(*types.Func); isFunc && strings.HasPrefix(sel.Sel.Name, "New") && obj.Pkg() != nil &&
				strings.HasSuffix(obj.Pkg().Path(), "/adapter/jsonfile") && f.pkg != "wire" && !strings.HasSuffix(f.pkg, "/adapter/jsonfile") {
				out = append(out, finding{"one-writer-per-store", f.at(call.Pos()), fmt.Sprintf("%q builds a store: only wire constructs one", f.pkg)})
			}
			if !storeWrite(sel.Sel.Name) {
				return true
			}
			store, ok := storeOf(f, sel)
			if !ok {
				return true
			}
			if store == preferences && sel.Sel.Name == "Write" && restOf(f.pkg) == "contract" && topOf(f.pkg) != "core" && len(call.Args) > 0 {
				if tv, ok := f.info.Types[call.Args[0]]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
					group := constant.StringVal(tv.Value)
					if other, taken := groups[group]; taken && other != f.pkg {
						out = append(out, finding{"one-writer-per-store", f.at(call.Pos()), fmt.Sprintf("the preference group %q is written by %q and %q: one owner per group", group, other, f.pkg)})
					}
					groups[group] = f.pkg
				}
			}
			if mayWrite(f.pkg, store) {
				return true
			}
			key := f.rel + " " + store
			if storeWritersTolerated()[key] != "" {
				seen[key] = true
				return true
			}
			out = append(out, finding{"one-writer-per-store", f.at(call.Pos()), fmt.Sprintf("%q writes %s: only %s does", f.pkg, strings.TrimPrefix(store, module+"/src/"), storeOwners()[store])})
			return true
		})
	}
	for _, key := range slices.Sorted(maps.Keys(storeWritersTolerated())) {
		if !seen[key] {
			out = append(out, finding{"one-writer-per-store", "specs/architecture/stores_test.go", key + " no longer writes its store: remove it from storeWritersTolerated"})
		}
	}
	return out
}

func TestOneWriterPerStore(t *testing.T)      { judge(t, "OneWriterPerStore") }
func TestOneWriterPerStoreBuilt(t *testing.T) { judge(t, "OneWriterPerStoreBuilt") }
func TestOneWriterPerStoreGroup(t *testing.T) { judge(t, "OneWriterPerStoreGroup") }

func TestOneWriterPerStoreAllowlistShrinks(t *testing.T) {
	clean, _ := worlds(t)
	key := slices.Sorted(maps.Keys(storeWritersTolerated()))[0]
	rel, _, _ := strings.Cut(key, " ")
	gone := &world{root: clean.root, packages: clean.packages, cgo: clean.cgo}
	for _, f := range clean.files {
		if f.rel != rel {
			gone.files = append(gone.files, f)
		}
	}
	if !slices.ContainsFunc(ruleOneWriterPerStore(gone), func(f finding) bool { return strings.HasPrefix(f.what, key+" no longer writes") }) {
		t.Errorf("the one-writer-per-store rule kept the line of %s, a file that is gone", rel)
	}
}
