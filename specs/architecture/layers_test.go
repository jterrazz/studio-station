package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/jterrazz/studio-station"

const (
	cobra  = "github.com/spf13/cobra"
	pflag  = "github.com/spf13/pflag"
	fang   = "charm.land/fang/v2"
	lip    = "charm.land/lipgloss/v2"
	huh    = "charm.land/huh/v2"
	tea    = "charm.land/bubbletea/v2"
	bubble = "charm.land/bubbles/v2"
	term   = "github.com/charmbracelet/x/term"
	ansi   = "github.com/charmbracelet/x/ansi"
	cprof  = "github.com/charmbracelet/colorprofile"
	std    = "std"
)

// invariant: the import table of docs/concepts/architecture.md, owned HERE. A
// package matches the row with the most specific pattern: an exact path beats
// `x/*`, a longer prefix beats a shorter. `std` is the standard library minus
// forbiddenStd; a forbidden member is re-allowed only by its name in a row.
func importTable() map[string][]string {
	domain := []string{"system", "catalog", "domain/settings", std}
	return map[string][]string{
		"cmd/station":       {"app", "os"},
		"cmd/skillsgen":     {"catalog", "os", "flag", std},
		"app":               {"*", fang, cobra, lip, term, "os", "os/signal", "runtime/debug", std},
		"cli":               {"cli/kit", cobra, std},
		"cli/kit":           {"output", "present", "prompt", "fail", "domain/*", "catalog", cobra, pflag, std},
		"cli/*":             {"cli/kit", "domain/*", "present", "present/*", "catalog", "prompt", cobra, pflag, std},
		"domain/*":          domain,
		"domain/setup":      append(slices.Clone(domain), "domain/shell"),
		"domain/status":     append(slices.Clone(domain), "domain/tools", "domain/config", "domain/tailnet", "domain/setup"),
		"system":            {std},
		"system/tart":       {"system", std},
		"system/host":       {"system", "os", "os/exec", "os/user", "net", "net/http", "syscall", std},
		"system/systemtest": {"system", "testing", "testing/fstest", std},
		"catalog":           {"embed", std},
		"present":           {"catalog", std},
		"present/*":         {"present", "present/*", "domain/*", "catalog", std},
		"output":            {"present", "theme", "fail", lip, cprof, ansi, std},
		"theme":             {lip, std},
		"theme/forms":       {"theme", huh, std},
		"prompt":            {"theme", "theme/forms", "catalog", huh, std},
		"fail":              {"testing", std},
		"tui":               {"tui/kit", "tui/screens/*", "domain/*", "present", "present/*", "output", "theme", "theme/forms", "catalog", "fail", tea, bubble, huh, lip, std},
		"tui/*":             {"tui/kit", "domain/*", "present", "present/*", "output", "theme", "theme/forms", "catalog", "fail", tea, bubble, huh, lip, std},
	}
}

func forbiddenStd(path string) bool {
	switch path {
	case "os", "os/exec", "os/signal", "os/user", "net", "net/http", "syscall", "io/ioutil",
		"flag", "log", "testing", "testing/fstest", "unsafe", "plugin", "runtime/debug":
		return true
	}
	return strings.HasPrefix(path, "net/") || strings.HasPrefix(path, "testing/")
}

var testImports = []string{"testing", "testing/fstest", "flag", "os", "path/filepath", "system/systemtest"}

type symbolRule struct {
	symbols []string
	owners  []string
	tests   bool
}

// invariant: the forbidden symbols, matched on the resolved object as
// `<package path>.<name>` or `<package path>.<receiver type>.<name>`; a `*`
// symbol stands for every member of its package. The first matching rule
// judges. A symbol is always allowed inside its own package.
func symbolRules() []symbolRule {
	return []symbolRule{
		{symbols: []string{"os.Getenv", "os.LookupEnv", "os.Setenv", "os.UserHomeDir", "os.Hostname", "os.Geteuid"}, owners: []string{"system/host"}},
		{symbols: []string{"os.Environ", "os.Args", "os.Exit", "os.Stdin", "os.Stdout", "os.Stderr"}, owners: []string{"cmd/station"}},
		{symbols: []string{"os.ReadFile", "os.WriteFile", "os.MkdirAll"}, owners: []string{"system/host", "cmd/skillsgen"}, tests: true},
		{symbols: []string{"os.File", "os.File.Fd", "os.Interrupt"}, owners: []string{"system/host", "app"}},
		{symbols: []string{"os.*", "os/exec.*", "syscall.*", "os/user.*"}, owners: []string{"system/host"}},
		{symbols: []string{"net.*", "net/http.*"}, owners: []string{"system/host"}},
		{symbols: []string{"fmt.Print", "fmt.Println", "fmt.Printf", "log.*", "print", "println"}},
		{symbols: []string{"fmt.Fprint", "fmt.Fprintln", "fmt.Fprintf", "io.WriteString", "io.Copy", "io.CopyN"}, owners: []string{"output", "prompt", "system/host", "cmd/skillsgen"}},
		{symbols: cobraWriters(), owners: []string{"cli/kit", "app"}},
		{symbols: []string{"time.Now", "time.Since", "time.Until", "time.Sleep", "time.After", "time.Tick", "time.NewTimer", "time.NewTicker", "time.AfterFunc"}, owners: []string{"system/host", "output", "tui", "tui/*"}},
		{symbols: []string{"path/filepath.Glob", "path/filepath.Walk", "path/filepath.WalkDir", "path/filepath.EvalSymlinks", "path/filepath.Abs", "io/fs.WalkDir", "io/fs.Glob"}, owners: []string{"system/host", "system/systemtest"}},
		{symbols: paletteSymbols(), owners: []string{"theme"}},
		{symbols: []string{lip + ".*"}, owners: []string{"output", "theme", "tui", "tui/*", "app"}},
		{symbols: []string{tea + ".*", bubble + "/*"}, owners: []string{"tui", "tui/*"}},
		{symbols: []string{huh + ".*"}, owners: []string{"theme/forms", "prompt", "tui/kit"}},
		{symbols: []string{fang + ".*"}, owners: []string{"app"}},
		{symbols: prefixed(module+"/src/fail.", "Usage", "Failed", "Silent", "Interrupted"), owners: []string{"cli/kit", "app"}},
		{symbols: append(prefixed(module+"/src/output.", "NewWriter", "Detect", "For"), module+"/src/output.Writer.*"), owners: []string{"cli/kit", "app"}},
		{symbols: prefixed(module+"/src/output.", "TTYString", "TTYBody"), owners: []string{"tui", "tui/*"}},
	}
}

func cobraWriters() []string {
	return prefixed(cobra+".Command.", "Print", "Println", "Printf", "PrintErr", "PrintErrln", "PrintErrf",
		"OutOrStdout", "OutOrStderr", "ErrOrStderr", "InOrStdin", "SetOut", "SetErr", "SetIn", "SetOutput")
}

func paletteSymbols() []string {
	return []string{lip + ".Color", lip + ".RGBColor", "image/color.RGBA", "image/color.NRGBA"}
}

func prefixed(prefix string, names ...string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return out
}

var (
	glyphs = []string{"●", "◐", "○", "✗", "✓", "✅", "⚠", "❌", "ℹ", "？", "▶", "▾", "▸"}
	hexRe  = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
)

type file struct {
	pkg     string
	path    string
	rel     string
	test    bool
	syntax  *ast.File
	info    *types.Info
	types   *types.Package
	fset    *token.FileSet
	imports []string
}

type finding struct{ rule, where, what string }

type world struct {
	root     string
	files    []*file
	packages map[string]bool
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func load(t *testing.T, overlay map[string]string) *world {
	t.Helper()
	root := repoRoot(t)
	cfg := &packages.Config{
		Dir:     root,
		Tests:   true,
		Mode:    packages.NeedName | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles | packages.NeedCompiledGoFiles,
		Overlay: map[string][]byte{},
	}
	for name, src := range overlay {
		cfg.Overlay[filepath.Join(root, name)] = []byte(src)
	}
	pkgs, err := packages.Load(cfg, "./src/...")
	if err != nil {
		t.Fatal(err)
	}
	w := &world{root: root, packages: map[string]bool{}}
	seen := map[string]bool{}
	for _, p := range pkgs {
		if strings.HasSuffix(p.ID, ".test") {
			continue
		}
		for _, e := range p.Errors {
			t.Errorf("%s: %v", p.ID, e)
		}
		rel, ok := strings.CutPrefix(strings.TrimSuffix(p.PkgPath, "_test"), module+"/src/")
		if !ok {
			continue
		}
		w.packages[rel] = true
		for i, syntax := range p.Syntax {
			path := p.CompiledGoFiles[i]
			if seen[path] {
				continue
			}
			seen[path] = true
			f := &file{pkg: rel, path: path, test: strings.HasSuffix(path, "_test.go"), syntax: syntax, info: p.TypesInfo, types: p.Types, fset: p.Fset}
			f.rel, _ = filepath.Rel(root, path)
			for _, spec := range syntax.Imports {
				ip, _ := strconv.Unquote(spec.Path.Value)
				f.imports = append(f.imports, ip)
			}
			w.files = append(w.files, f)
		}
	}
	return w
}

func matches(pattern, pkg string) bool {
	if pattern == "*" {
		return true
	}
	if base, ok := strings.CutSuffix(pattern, "/*"); ok {
		return strings.HasPrefix(pkg, base+"/")
	}
	return pattern == pkg
}

func rowOf(pkg string) (string, bool) {
	best, score := "", -1
	for pattern := range importTable() {
		if !matches(pattern, pkg) {
			continue
		}
		s := len(pattern) * 2
		if !strings.HasSuffix(pattern, "/*") {
			s = 1 << 20
		}
		if s > score {
			best, score = pattern, s
		}
	}
	return best, score >= 0
}

func (f *file) at(pos token.Pos) string {
	p := f.fset.Position(pos)
	return fmt.Sprintf("%s:%d", f.rel, p.Line)
}

func allowedImport(row, pkg, ip string, test bool) bool {
	allowed := importTable()[row]
	if test {
		allowed = append(slices.Clone(allowed), testImports...)
		if ip == module+"/src/"+pkg {
			return true
		}
	}
	if rel, ok := strings.CutPrefix(ip, module+"/src/"); ok {
		for _, a := range allowed {
			if matches(a, rel) {
				return true
			}
		}
		return false
	}
	first, _, _ := strings.Cut(ip, "/")
	if !strings.Contains(first, ".") {
		if slices.Contains(allowed, ip) {
			return true
		}
		return slices.Contains(allowed, std) && !forbiddenStd(ip)
	}
	for _, a := range allowed {
		if ip == a || strings.HasPrefix(ip, a+"/") {
			return true
		}
	}
	return false
}

func ruleRows(w *world) []finding {
	var out []finding
	for _, pkg := range sortedKeys(w.packages) {
		if _, ok := rowOf(pkg); !ok {
			out = append(out, finding{"rows", "src/" + pkg, "no row of the import table names this package"})
		}
	}
	return out
}

func ruleImports(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		row, ok := rowOf(f.pkg)
		if !ok {
			continue
		}
		for i, ip := range f.imports {
			if !allowedImport(row, f.pkg, ip, f.test) {
				out = append(out, finding{"imports", f.at(f.syntax.Imports[i].Pos()), fmt.Sprintf("row %q may not import %q", row, ip)})
			}
		}
	}
	return out
}

func objectKey(obj types.Object) (key, pkg string) {
	if b, ok := obj.(*types.Builtin); ok {
		return b.Name(), ""
	}
	if obj.Pkg() == nil {
		return "", ""
	}
	pkg = obj.Pkg().Path()
	if fn, ok := obj.(*types.Func); ok {
		if recv := fn.Signature().Recv(); recv != nil {
			t := recv.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if named, ok := t.(*types.Named); ok {
				return pkg + "." + named.Obj().Name() + "." + obj.Name(), pkg
			}
			return "", pkg
		}
	}
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return "", pkg
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return "", pkg
	}
	return pkg + "." + obj.Name(), pkg
}

func symbolMatches(symbol, key, pkg string) bool {
	if base, ok := strings.CutSuffix(symbol, ".*"); ok {
		return pkg == base
	}
	if base, ok := strings.CutSuffix(symbol, "/*"); ok {
		return pkg == base || strings.HasPrefix(pkg, base+"/")
	}
	return symbol == key
}

func ruleSymbols(w *world) []finding {
	var out []finding
	rules := symbolRules()
	for _, f := range w.files {
		self := module + "/src/" + f.pkg
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := f.info.Uses[id]
			if obj == nil {
				return true
			}
			key, pkg := objectKey(obj)
			if key == "" || pkg == self {
				return true
			}
			for _, r := range rules {
				if !slices.ContainsFunc(r.symbols, func(s string) bool { return symbolMatches(s, key, pkg) }) {
					continue
				}
				if (r.tests && f.test) || slices.ContainsFunc(r.owners, func(o string) bool { return matches(o, f.pkg) }) {
					return true
				}
				out = append(out, finding{"symbols", f.at(id.Pos()), fmt.Sprintf("%s is not allowed in %q", key, f.pkg)})
				return true
			}
			return true
		})
	}
	return out
}

func ruleSystemtest(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		for i, ip := range f.imports {
			if ip == module+"/src/system/systemtest" && !f.test {
				out = append(out, finding{"systemtest", f.at(f.syntax.Imports[i].Pos()), "system/systemtest is imported outside a _test.go file"})
			}
		}
	}
	return out
}

func literals(f *file, visit func(lit *ast.BasicLit, value string)) {
	ast.Inspect(f.syntax, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || (lit.Kind != token.STRING && lit.Kind != token.CHAR) {
			return true
		}
		value := lit.Value
		if lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				value = v
			}
		} else if r, _, _, err := strconv.UnquoteChar(strings.Trim(lit.Value, "'"), '\''); err == nil {
			value = string(r)
		}
		visit(lit, value)
		return true
	})
}

func ruleANSI(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		literals(f, func(lit *ast.BasicLit, value string) {
			if strings.Contains(value, "\x1b[") || strings.Contains(lit.Value, `\x1b[`) || strings.Contains(lit.Value, `\033[`) {
				out = append(out, finding{"ansi", f.at(lit.Pos()), "an ANSI escape literal: styles come from theme, erasures from x/ansi"})
			}
		})
	}
	return out
}

func rulePalette(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.pkg == "theme" {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			if hexRe.MatchString(value) {
				out = append(out, finding{"palette", f.at(lit.Pos()), "a hex color outside theme"})
			}
		})
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if obj := f.info.Uses[id]; obj != nil {
					key, _ := objectKey(obj)
					if slices.Contains(paletteSymbols(), key) {
						out = append(out, finding{"palette", f.at(id.Pos()), key + " outside theme"})
					}
				}
			}
			return true
		})
	}
	return out
}

func ruleMarks(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.rel == filepath.Join("src", "present", "mark.go") {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			for _, g := range glyphs {
				if strings.Contains(value, g) {
					out = append(out, finding{"marks", f.at(lit.Pos()), fmt.Sprintf("the glyph %q outside src/present/mark.go", g)})
				}
			}
		})
	}
	return out
}

func isFunc(t types.Type) bool {
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

func ruleCatalog(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.pkg != "catalog" || f.test {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncLit:
				out = append(out, finding{"catalog", f.at(v.Pos()), "a func literal in the catalogue"})
			case *ast.StructType:
				for _, field := range v.Fields.List {
					if t := f.info.TypeOf(field.Type); t != nil && isFunc(t) {
						out = append(out, finding{"catalog", f.at(field.Pos()), "a func-typed field in the catalogue"})
					}
				}
			}
			return true
		})
		for _, decl := range f.syntax.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if obj := f.info.Defs[name]; obj != nil && isFunc(obj.Type()) {
						out = append(out, finding{"catalog", f.at(name.Pos()), "a func-typed package-level value in the catalogue"})
					}
				}
			}
		}
	}
	return out
}

func ruleWorded(w *world) []finding {
	worded := map[string]bool{}
	for _, f := range w.files {
		if f.pkg != "cli/kit" || !strings.HasPrefix(filepath.Base(f.path), "failures_") {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "As" {
				return true
			}
			if obj := f.info.Uses[sel.Sel]; obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "errors" {
				return true
			}
			t := f.info.TypeOf(call.Args[1])
			for {
				p, ok := t.(*types.Pointer)
				if !ok {
					break
				}
				t = p.Elem()
			}
			if named, ok := t.(*types.Named); ok && named.Obj().Pkg() != nil {
				worded[named.Obj().Pkg().Path()+"."+named.Obj().Name()] = true
			}
			return true
		})
	}
	var out []finding
	for _, f := range w.files {
		if !strings.HasPrefix(f.pkg, "domain/") || f.test {
			continue
		}
		for _, decl := range f.syntax.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if !ts.Name.IsExported() || !strings.HasSuffix(ts.Name.Name, "Error") {
					continue
				}
				if !worded[f.types.Path()+"."+ts.Name.Name] {
					out = append(out, finding{"worded", f.at(ts.Pos()), ts.Name.Name + " is no errors.As target of src/cli/kit/failures_*.go"})
				}
			}
		}
	}
	return out
}

var kinds = []string{"invariant:", "hazard:", "outside:", "value:"}

func commentFindings(fset *token.FileSet, rel string, syntax *ast.File) []finding {
	if ast.IsGenerated(syntax) {
		return nil
	}
	var out []finding
	for _, group := range syntax.Comments {
		first := group.List[0].Text
		if strings.HasPrefix(first, "//go:") || strings.HasPrefix(first, "//nolint") {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(first, "//"), "/*"))
		if !slices.ContainsFunc(kinds, func(k string) bool { return strings.HasPrefix(text, k) }) {
			out = append(out, finding{"comments", fmt.Sprintf("%s:%d", rel, fset.Position(group.Pos()).Line), "a comment that does not open with invariant:, hazard:, outside: or value:"})
		}
	}
	return out
}

func ruleComments(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		out = append(out, commentFindings(f.fset, f.rel, f.syntax)...)
	}
	dir := filepath.Join(w.root, "specs", "architecture")
	entries, _ := os.ReadDir(dir)
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		syntax, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			out = append(out, finding{"comments", e.Name(), err.Error()})
			continue
		}
		out = append(out, commentFindings(fset, filepath.Join("specs", "architecture", e.Name()), syntax)...)
	}
	return out
}

func ruleRegistries(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		for _, decl := range f.syntax.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "init" {
					out = append(out, finding{"registries", f.at(d.Pos()), "an init() function"})
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR || f.pkg == "catalog" {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range vs.Names {
						obj := f.info.Defs[name]
						if obj == nil {
							continue
						}
						switch obj.Type().Underlying().(type) {
						case *types.Slice, *types.Map:
							out = append(out, finding{"registries", f.at(name.Pos()), "a package-level slice or map: a mutable registry"})
						}
					}
				}
			}
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

type rule struct {
	name    string
	check   func(*world) []finding
	mutant  string
	source  string
	expects string
}

func rules() []rule {
	return []rule{
		{"Rows", ruleRows, "src/mutant/mutant.go", "package mutant\n", "src/mutant"},
		{"Imports", ruleImports, "src/system/mutant_imports.go", "package system\n\nimport _ \"os/exec\"\n", "src/system/mutant_imports.go"},
		{"Symbols", ruleSymbols, "src/present/mutant_symbols.go", "package present\n\nimport \"time\"\n\nfunc mutantNow() time.Time { return time.Now() }\n", "src/present/mutant_symbols.go"},
		{"Systemtest", ruleSystemtest, "src/theme/mutant_systemtest.go", "package theme\n\nimport _ \"" + module + "/src/system/systemtest\"\n", "src/theme/mutant_systemtest.go"},
		{"ANSI", ruleANSI, "src/present/mutant_ansi.go", "package present\n\nconst mutantBold = \"\\x1b[1m\"\n", "src/present/mutant_ansi.go"},
		{"Palette", rulePalette, "src/present/mutant_palette.go", "package present\n\nconst mutantRed = \"#ff0000\"\n", "src/present/mutant_palette.go"},
		{"Marks", ruleMarks, "src/output/mutant_marks.go", "package output\n\nconst mutantTick = \"✓ done\"\n", "src/output/mutant_marks.go"},
		{"Catalog", ruleCatalog, "src/catalog/mutant_catalog.go", "package catalog\n\ntype mutantEntry struct{ Check func() bool }\n\nvar mutantDefault = mutantEntry{Check: func() bool { return true }}\n", "src/catalog/mutant_catalog.go"},
		{"Worded", ruleWorded, "src/domain/mutant/mutant.go", "package mutant\n\ntype UnwordedError struct{}\n\nfunc (UnwordedError) Error() string { return \"unworded\" }\n", "src/domain/mutant/mutant.go"},
		{"Comments", ruleComments, "src/present/mutant_comments.go", "package present\n\n// a comment that states no kind\nconst mutantComment = 1\n", "src/present/mutant_comments.go"},
		{"Registries", ruleRegistries, "src/present/mutant_registries.go", "package present\n\nvar mutantRegistry = map[string]int{}\n\nfunc init() { mutantRegistry[\"x\"] = 1 }\n", "src/present/mutant_registries.go"},
	}
}

var (
	loadOnce                sync.Once
	cleanWorld, mutantWorld *world
)

func worlds(t *testing.T) (clean, mutant *world) {
	t.Helper()
	loadOnce.Do(func() {
		overlay := map[string]string{}
		for _, r := range rules() {
			overlay[r.mutant] = r.source
		}
		cleanWorld, mutantWorld = load(t, nil), load(t, overlay)
	})
	if cleanWorld == nil || mutantWorld == nil {
		t.Fatal("the packages under src/ did not load")
	}
	return cleanWorld, mutantWorld
}

func judge(t *testing.T, name string) {
	clean, mutant := worlds(t)
	i := slices.IndexFunc(rules(), func(r rule) bool { return r.name == name })
	r := rules()[i]
	t.Run("holds", func(t *testing.T) {
		for _, f := range r.check(clean) {
			t.Errorf("%s: %s", f.where, f.what)
		}
	})
	t.Run("bites", func(t *testing.T) {
		for _, f := range r.check(mutant) {
			if strings.HasPrefix(f.where, r.expects) {
				return
			}
		}
		t.Errorf("the %s rule did not report the violating file %s", r.name, r.mutant)
	})
}

func TestRows(t *testing.T)       { judge(t, "Rows") }
func TestImports(t *testing.T)    { judge(t, "Imports") }
func TestSymbols(t *testing.T)    { judge(t, "Symbols") }
func TestSystemtest(t *testing.T) { judge(t, "Systemtest") }
func TestANSI(t *testing.T)       { judge(t, "ANSI") }
func TestPalette(t *testing.T)    { judge(t, "Palette") }
func TestMarks(t *testing.T)      { judge(t, "Marks") }
func TestCatalog(t *testing.T)    { judge(t, "Catalog") }
func TestWorded(t *testing.T)     { judge(t, "Worded") }
func TestComments(t *testing.T)   { judge(t, "Comments") }
func TestRegistries(t *testing.T) { judge(t, "Registries") }
