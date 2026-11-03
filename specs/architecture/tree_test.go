package architecture_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// invariant: ARCHITECTURE 13.1: the top level of src/ names what station does.
func families() []string {
	return []string{"machines", "activity", "apps", "settings", "disk", "ports", "accounts", "background", "awake", "images", "overview", "setup", "engine"}
}

func plumbing() []string {
	return []string{"core", "remote", "surface", "platform", "world", "wire", "cmd", "catalog", "page", "pages", "ui", "output"}
}

func presentation() []string {
	return []string{"page", "pages", "ui", "output"}
}

func sectioned() []string {
	return []string{"overview", "activity", "apps", "settings", "disk", "ports", "accounts", "background", "awake"}
}

// invariant: the allowlist of the tree: today's top-level packages that are
// neither a family nor plumbing. It only shrinks; a line whose package is
// gone fails the gate until it is removed.
func toleratedTop() []string {
	return []string{"domain", "system"}
}

func topOf(pkg string) string {
	top, _, _ := strings.Cut(pkg, "/")
	return top
}

func restOf(pkg string) string {
	_, rest, _ := strings.Cut(pkg, "/")
	return rest
}

func familyInside(family, rest string) bool {
	switch rest {
	case "", "contract", "contract/contracttest", "calls":
		return true
	case "process", "jobs", "feeds":
		return family == "engine"
	}
	tech, ok := strings.CutPrefix(rest, "adapter/")
	return ok && tech != "" && !strings.Contains(tech, "/")
}

// invariant: rule tree-screams: every top-level package is a family or named
// plumbing; a family holds only its model, contract/, calls/ and
// adapter/<tech>/ (the engine adds process/, jobs/ and feeds/); a family with
// a section has an applies.go once it holds more than its doc.go.
func ruleTree(w *world) []finding {
	var out []finding
	tops := map[string]bool{}
	applies, filled := map[string]bool{}, map[string]bool{}
	for _, f := range w.files {
		if f.test {
			continue
		}
		switch filepath.Base(f.path) {
		case "applies.go":
			applies[f.pkg] = true
		case "doc.go":
		default:
			filled[topOf(f.pkg)] = true
		}
	}
	for _, pkg := range sortedKeys(w.packages) {
		top := topOf(pkg)
		tops[top] = true
		switch {
		case slices.Contains(families(), top):
			if !familyInside(top, restOf(pkg)) {
				out = append(out, finding{"tree-screams", "src/" + pkg, "a family holds only its model, contract/, calls/ and adapter/<tech>/"})
			}
		case slices.Contains(plumbing(), top), slices.Contains(toleratedTop(), top):
		default:
			out = append(out, finding{"tree-screams", "src/" + pkg, "a top-level package that is neither a family nor named plumbing"})
		}
	}
	for _, family := range sectioned() {
		if filled[family] && !applies[family] {
			out = append(out, finding{"tree-screams", "src/" + family, "a family with a section has an applies.go"})
		}
	}
	for _, top := range toleratedTop() {
		if !tops[top] {
			out = append(out, finding{"tree-screams", "specs/architecture/tree_test.go", "src/" + top + " is gone: remove its line from toleratedTop"})
		}
	}
	return out
}

func TestTreeScreams(t *testing.T) { judge(t, "TreeScreams") }
func TestTreeInside(t *testing.T)  { judge(t, "TreeInside") }
func TestTreeApplies(t *testing.T) { judge(t, "TreeApplies") }

func TestTreeAllowlistShrinks(t *testing.T) {
	clean, _ := worlds(t)
	top := toleratedTop()[0]
	got := ruleTree(withoutPackage(clean, func(pkg string) bool { return topOf(pkg) == top }))
	if !slices.ContainsFunc(got, func(f finding) bool { return strings.Contains(f.what, "src/"+top+" is gone") }) {
		t.Errorf("the tree rule kept the line of src/%s, a package that is gone", top)
	}
}
