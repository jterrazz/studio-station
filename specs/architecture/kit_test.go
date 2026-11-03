package architecture_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	uitoken "github.com/jterrazz/studio-station/src/ui/token"
)

// invariant: the packages the design-system lot has not migrated yet, judged
// by the old rules only. The list only shrinks: a package leaves it in the
// commit that moves it onto src/page and src/ui, and the next maker empties it.
func notYetMigrated() []string {
	return []string{
		"app",
		"cli/config", "cli/config/*", "cli/daemon", "cli/guest", "cli/images", "cli/images/*",
		"cli/machines", "cli/machines/*", "cli/run", "cli/run/*", "cli/secrets", "cli/secrets/*",
		"cli/services", "cli/services/*", "cli/setup", "cli/shell", "cli/status", "cli/storage", "cli/storage/*",
		"cli/tailnet", "cli/tools/install", "cli/tools/uninstall", "cli/tools/upgrade",
		"domain/services", "domain/setup", "domain/status", "domain/tools",
		"output", "present", "present/*", "prompt", "theme", "theme/forms",
		"tui", "tui/kit", "tui/screens/config", "tui/screens/fleet", "tui/screens/status", "tui/screens/storage",
	}
}

func migrated(pkg string) bool {
	return !slices.ContainsFunc(notYetMigrated(), func(p string) bool { return matches(p, pkg) })
}

func under(pkg string, patterns ...string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return matches(p, pkg) })
}

const qr = "rsc.io/qr"

// invariant: rule 13: the drawing libraries are the kit's; fang is help's and
// the entry point's, Bubble Tea the program's and the TUI renderer's.
func ruleKit(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if !migrated(f.pkg) {
			continue
		}
		for i, ip := range f.imports {
			lib := func(p string) bool { return ip == p || strings.HasPrefix(ip, p+"/") }
			ok := true
			switch {
			case lib(lip), lib(bubble), lib(huh), lib(ansi), lib(cprof), lib(term), lib(qr):
				ok = under(f.pkg, "ui/*")
			case lib(fang):
				ok = under(f.pkg, "ui/help", "app")
			case lib(tea):
				ok = under(f.pkg, "tui", "tui/screens/*", "ui/render/tui")
			}
			if !ok {
				out = append(out, finding{"kit", f.at(f.syntax.Imports[i].Pos()), fmt.Sprintf("%q may not import %q: drawing is the kit's", f.pkg, ip)})
			}
		}
	}
	return out
}

func levels() []string {
	return []string{"ui/token", "ui/atom", "ui/molecule", "ui/organism", "ui/template", "ui/render"}
}

func levelOf(pkg string) int {
	for i, l := range slices.Backward(levels()) {
		if pkg == l || strings.HasPrefix(pkg, l+"/") {
			return i
		}
	}
	return -1
}

// invariant: rule 14: a kit level imports no level above its own, and the kit
// renders page nodes only, never the domain, a page function or the catalogue.
func ruleLevels(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		own := levelOf(f.pkg)
		if own < 0 {
			continue
		}
		for i, ip := range f.imports {
			rel, ok := strings.CutPrefix(ip, module+"/src/")
			if !ok {
				continue
			}
			switch {
			case under(rel, "domain/*", "pages/*", "catalog", "system", "system/*"):
				out = append(out, finding{"levels", f.at(f.syntax.Imports[i].Pos()), f.pkg + " renders page nodes only, not " + rel})
			case levelOf(rel) > own && (!f.test || f.pkg != "ui/render"):
				out = append(out, finding{"levels", f.at(f.syntax.Imports[i].Pos()), f.pkg + " imports " + rel + ", a level above its own"})
			}
		}
	}
	return out
}

var widthVerb = regexp.MustCompile(`%[-+ #0]*(\*|[1-9]\d*)(\.\d+)?[a-zA-Z]`)

func callee(f *file, call *ast.CallExpr) string {
	var id *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		id = fn.Sel
	case *ast.Ident:
		id = fn
	default:
		return ""
	}
	obj := f.info.Uses[id]
	if obj == nil {
		return ""
	}
	key, _ := objectKey(obj)
	return key
}

func paddedFormat(f *file, call *ast.CallExpr, key string) bool {
	at := map[string]int{"fmt.Sprintf": 0, "fmt.Printf": 0, "fmt.Fprintf": 1}
	i, ok := at[key]
	if !ok || len(call.Args) <= i {
		return false
	}
	tv, ok := f.info.Types[call.Args[i]]
	if !ok || tv.Value == nil {
		return false
	}
	s, err := strconv.Unquote(tv.Value.ExactString())
	return err == nil && widthVerb.MatchString(s)
}

// invariant: rule 15: a page function returns meaning (a page, a node, a
// contract value), never text, and lays nothing out.
func ruleSemantic(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if !under(f.pkg, "pages/*") || f.test {
			continue
		}
		for _, decl := range f.syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			for _, r := range fn.Type.Results.List {
				if t := f.info.TypeOf(r.Type); t != nil && !semantic(t) {
					out = append(out, finding{"semantic", f.at(fn.Pos()), fn.Name.Name + " returns " + t.String() + ": a page function returns page nodes"})
				}
			}
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if key := callee(f, call); key == "strings.Repeat" || paddedFormat(f, call, key) {
				out = append(out, finding{"semantic", f.at(call.Pos()), key + " lays out text in a page function"})
			}
			return true
		})
	}
	return out
}

func semantic(t types.Type) bool {
	for {
		switch u := t.(type) {
		case *types.Slice:
			t = u.Elem()
			continue
		case *types.Pointer:
			t = u.Elem()
			continue
		case *types.Array:
			t = u.Elem()
			continue
		}
		break
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	path := named.Obj().Pkg().Path()
	if path == module+"/src/page" {
		return true
	}
	return strings.HasPrefix(path, module+"/src/pages/") && strings.HasSuffix(named.Obj().Name(), "JSON")
}

func bannedRunes() []rune {
	var out []rune
	for _, g := range append(uitoken.Glyphs(), "✓✅⚠❌ℹ？▶▾▸") {
		for _, r := range g {
			if r > 127 && !slices.Contains(out, r) {
				out = append(out, r)
			}
		}
	}
	return out
}

// invariant: rule 16: every glyph station prints is a token; the set is read
// from token.Glyphs itself, so a new glyph is banned the day it is declared.
func ruleGlyphs(w *world) []finding {
	var out []finding
	banned := bannedRunes()
	for _, f := range w.files {
		if f.pkg == "ui/token" || (f.test && under(f.pkg, "ui/*", "ui")) || !migrated(f.pkg) {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			for _, r := range value {
				if slices.Contains(banned, r) {
					out = append(out, finding{"glyphs", f.at(lit.Pos()), fmt.Sprintf("the glyph %q outside src/ui/token", r)})
					return
				}
			}
		})
	}
	return out
}

// invariant: rule 17: the page language carries meaning only; a field or type
// named for its presentation would let a page pick its own look.
func rulePresentation(w *world) []finding {
	var out []finding
	banned := []string{"Tone", "Color", "Style", "Width"}
	for _, f := range w.files {
		if f.pkg != "page" || f.test {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				if slices.Contains(banned, x.Name.Name) {
					out = append(out, finding{"presentation", f.at(x.Pos()), "the type " + x.Name.Name + " in the page language"})
				}
			case *ast.Field:
				for _, name := range x.Names {
					if slices.Contains(banned, name.Name) {
						out = append(out, finding{"presentation", f.at(name.Pos()), "the field " + name.Name + " in the page language"})
					}
				}
			}
			return true
		})
	}
	return out
}

func keyPress(f *file, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "String" {
		return false
	}
	t := f.info.TypeOf(sel.X)
	return t != nil && strings.HasSuffix(t.String(), "bubbletea/v2.KeyPressMsg")
}

// invariant: rule 19: a migrated tab binds no key of its own: its keys are its
// page's Actions and the Browser's key map, the two the key sheet is built from,
// so a key cannot exist without appearing on the sheet.
func ruleKeys(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if !under(f.pkg, "tui/screens/*") || !migrated(f.pkg) {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SwitchStmt:
				if x.Tag != nil && keyPress(f, x.Tag) {
					out = append(out, finding{"keys", f.at(x.Pos()), "a tab switches on a key itself, outside its page's Actions"})
				}
			case *ast.BinaryExpr:
				if keyPress(f, x.X) || keyPress(f, x.Y) {
					out = append(out, finding{"keys", f.at(x.Pos()), "a tab compares a key itself, outside its page's Actions"})
				}
			}
			return true
		})
	}
	return out
}

// invariant: rule 20: only page functions build pages; a tab or a leaf that
// assembles its own bypasses the one place a page is judged.
func ruleBuilders(w *world) []finding {
	var out []finding
	nodes := []string{"Page", "Section", "Row", "Group", "Item"}
	for _, f := range w.files {
		if f.test || under(f.pkg, "page", "pages/*", "cmd/generate", "cmd/generate/*") {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if named, ok := f.info.TypeOf(x).(*types.Named); ok && len(x.Elts) > 0 && named.Obj().Pkg() != nil &&
					named.Obj().Pkg().Path() == module+"/src/page" && slices.Contains(nodes, named.Obj().Name()) {
					out = append(out, finding{"builders", f.at(x.Pos()), "a page." + named.Obj().Name() + " built outside a page function"})
				}
			case *ast.CallExpr:
				if callee(f, x) == module+"/src/page.Seal" {
					out = append(out, finding{"builders", f.at(x.Pos()), "page.Seal called outside a page function"})
				}
			}
			return true
		})
	}
	return out
}

// invariant: rule 21: layout lives in the kit: outside src/ui nothing pads,
// measures or repeats a line, and only the program wraps a view.
func ruleLayout(w *world) []finding {
	var out []finding
	measures := []string{"strings.Repeat", lip + ".Width", ansi + ".StringWidth"}
	for _, f := range w.files {
		// value: cmd/generate writes markdown for docs/, never station's output.
		if under(f.pkg, "ui/*", "ui", "cmd/generate", "cmd/generate/*") || f.test || !migrated(f.pkg) {
			continue
		}
		for i, ip := range f.imports {
			if ip == "text/tabwriter" {
				out = append(out, finding{"layout", f.at(f.syntax.Imports[i].Pos()), "text/tabwriter outside the kit"})
			}
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			key := callee(f, call)
			switch {
			case slices.Contains(measures, key), paddedFormat(f, call, key):
				out = append(out, finding{"layout", f.at(call.Pos()), key + " lays out text outside the kit"})
			case key == tea+".NewView" && f.pkg != "tui":
				out = append(out, finding{"layout", f.at(call.Pos()), "tea.NewView outside the program's View"})
			}
			return true
		})
	}
	return out
}

// invariant: rule 18: the renderers every fixture is proved by, the golden
// names src/ui/render/render_test.go writes.
func goldensOf(fixture string) []string {
	out := []string{"tty100", "tty80", "pipe", "json", "tui100", "tui80"}
	if strings.HasSuffix(fixture, ".full") {
		out = append(out, "ttylight100")
	}
	if strings.HasPrefix(fixture, "kitchen-sink.") {
		out = append(out, "tty40")
	}
	if strings.HasPrefix(fixture, "fleet.") || strings.HasPrefix(fixture, "services.") {
		out = append(out, "tui120")
	}
	return out
}

func missingGoldens(fixtures []string, goldens map[string]bool) []finding {
	var out []finding
	for _, fx := range fixtures {
		for _, r := range goldensOf(fx) {
			if !goldens[fx+"."+r] {
				out = append(out, finding{"goldens", "src/ui/testdata/__expected__/" + fx + "." + r, "a fixture without its golden"})
			}
		}
	}
	return out
}

func TestEveryFixtureHasItsGoldens(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "src", "ui", "testdata", "pages"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := make([]string, 0, len(entries))
	for _, e := range entries {
		fixtures = append(fixtures, strings.TrimSuffix(e.Name(), ".json"))
	}
	goldens := map[string]bool{}
	files, err := os.ReadDir(filepath.Join(root, "src", "ui", "testdata", "__expected__"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range files {
		goldens[e.Name()] = true
	}
	t.Run("holds", func(t *testing.T) {
		if len(fixtures) == 0 {
			t.Fatal("no page fixture under src/ui/testdata/pages")
		}
		for _, f := range missingGoldens(fixtures, goldens) {
			t.Errorf("%s: %s", f.where, f.what)
		}
	})
	t.Run("bites", func(t *testing.T) {
		without := map[string]bool{}
		for k := range goldens {
			without[k] = k != "tools.full.tui80"
		}
		if !slices.ContainsFunc(missingGoldens(fixtures, without), func(f finding) bool { return strings.HasSuffix(f.where, "tools.full.tui80") }) {
			t.Error("the goldens rule did not report a fixture without its tui80 golden")
		}
	})
}
