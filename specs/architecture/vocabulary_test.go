package architecture_test

import (
	"fmt"
	"go/ast"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// value: the words of SPEC §2.2 that the code and the docs keep for their
// own concept: the Tailnet contract, a file's path, the Power contract (the
// IOKit primitive), a run of blanks.
func codeWords() []string { return []string{"tailnet", "path", "power", "space"} }

func codeBanned() []banned {
	return slices.DeleteFunc(bannedWords(), func(b banned) bool {
		return slices.Contains(codeWords(), strings.ToLower(b.word))
	})
}

func saysBanned(list []banned, texts []string) []string {
	var found []string
	for _, b := range list {
		if len(bannedInList([]banned{b}, texts)) > 0 {
			found = append(found, b.word)
		}
	}
	return found
}

// value: where a text was read, as the allowlist keys it: its folder,
// repo-relative.
type corpusText struct{ dir, at, text string }

// invariant: a hidden alias says its old word on purpose (SPEC §4.3): the
// name kit.Alias registers and a function named for the alias it builds.
func aliasCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "Alias" || sel.Sel.Name == "AliasArgs")
}

func aliasName(name string) bool {
	return strings.HasSuffix(name, "Alias") || strings.HasSuffix(name, "Aliases")
}

func splitText(s string) string { return strings.Join(splitWords(s), " ") }

func sourceTexts(w *world) []corpusText {
	var out []corpusText
	for _, f := range w.files {
		if f.test {
			continue
		}
		dir := filepath.Dir(f.rel)
		base := strings.TrimSuffix(filepath.Base(f.rel), ".go")
		out = append(out, corpusText{dir, f.rel, splitText(base)}, corpusText{dir, f.rel, splitText(f.syntax.Name.Name)})
		skip := map[ast.Node]bool{}
		for _, spec := range f.syntax.Imports {
			skip[spec.Path] = true
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && aliasCall(call) {
				skip[call.Args[0]] = true
			}
			x, ok := n.(*ast.Ident)
			if !ok || x == f.syntax.Name || f.info.Defs[x] == nil || aliasName(x.Name) {
				return true
			}
			out = append(out, corpusText{dir, f.at(x.Pos()), splitText(x.Name)})
			return true
		})
		literals(f, func(lit *ast.BasicLit, value string) {
			if !skip[lit] {
				out = append(out, corpusText{dir, f.at(lit.Pos()), value})
			}
		})
	}
	return out
}

func walk(root, dir string, keep func(rel string) bool, read func(rel, body string)) error {
	var found []string
	err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if !d.IsDir() && keep(rel) {
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, rel := range found {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		read(rel, string(body))
	}
	return nil
}

var (
	fenceRe       = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe  = regexp.MustCompile("`[^`\n]*`")
	descriptionRe = regexp.MustCompile(`(?m)^description:\s*(.*)$`)
	swiftStringRe = regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`)
	swiftDeclRe   = regexp.MustCompile(`\b(?:struct|class|enum|protocol|func|let|var|case|typealias)\s+([A-Za-z_][A-Za-z0-9_]*)`)
)

// invariant: the docs read as prose: a code block or a code span quotes a
// command or a file as it is, the decision records are history.
func docTexts(root string) ([]corpusText, error) {
	var out []corpusText
	err := walk(root, "docs", func(rel string) bool {
		return strings.HasSuffix(rel, ".md") && !strings.HasPrefix(rel, "docs/decisions/")
	}, func(rel, body string) {
		prose := inlineCodeRe.ReplaceAllString(fenceRe.ReplaceAllString(body, ""), "")
		for i, line := range strings.Split(prose, "\n") {
			out = append(out, corpusText{filepath.Dir(rel), fmt.Sprintf("%s:%d", rel, i+1), line})
		}
	})
	return out, err
}

// invariant: a spec document says its words in its description and in its
// path; specs/usecases/migrate/ proves the hidden aliases by their old words.
func specTexts(root string) ([]corpusText, error) {
	var out []corpusText
	err := walk(root, "specs", func(rel string) bool {
		return strings.HasSuffix(rel, ".spec.yaml") && !strings.HasPrefix(rel, "specs/usecases/migrate/")
	}, func(rel, body string) {
		dir := filepath.Dir(rel)
		out = append(out, corpusText{dir, rel, splitText(strings.TrimSuffix(strings.TrimPrefix(rel, "specs/"), ".spec.yaml"))})
		for _, m := range descriptionRe.FindAllStringSubmatch(body, -1) {
			out = append(out, corpusText{dir, rel, m[1]})
		}
	})
	return out, err
}

func swiftTexts(root string) ([]corpusText, error) {
	var out []corpusText
	err := walk(root, "apps/menubar/sources", func(rel string) bool { return strings.HasSuffix(rel, ".swift") }, func(rel, body string) {
		dir := filepath.Dir(rel)
		for i, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			at := fmt.Sprintf("%s:%d", rel, i+1)
			for _, s := range swiftStringRe.FindAllString(line, -1) {
				out = append(out, corpusText{dir, at, strings.Trim(s, `"`)})
			}
			for _, m := range swiftDeclRe.FindAllStringSubmatch(line, -1) {
				out = append(out, corpusText{dir, at, splitText(m[1])})
			}
		}
	})
	return out, err
}

func vocabularyTexts(t *testing.T, w *world) []corpusText {
	t.Helper()
	out := sourceTexts(w)
	for _, read := range []func(string) ([]corpusText, error){docTexts, specTexts, swiftTexts} {
		texts, err := read(w.root)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, texts...)
	}
	return out
}

func vocabularySaid(texts []corpusText) (said, where map[string][]string) {
	said, where = map[string][]string{}, map[string][]string{}
	for _, c := range texts {
		for _, word := range saysBanned(codeBanned(), []string{c.text}) {
			if !slices.Contains(said[c.dir], word) {
				said[c.dir] = append(said[c.dir], word)
			}
			key := c.dir + " " + word
			where[key] = append(where[key], c.at)
		}
	}
	for _, words := range said {
		sort.Strings(words)
	}
	return said, where
}

func ruleVocabulary(texts []corpusText) []finding {
	return ruleVocabularyIn(texts, func(string) bool { return true })
}

func ruleVocabularyIn(texts []corpusText, in func(dir string) bool) []finding {
	said, where := vocabularySaid(texts)
	tolerated := maps.Clone(vocabularyTolerated())
	maps.DeleteFunc(tolerated, func(dir string, _ []string) bool { return !in(dir) })
	var out []finding
	for _, dir := range slices.Sorted(maps.Keys(said)) {
		for _, word := range said[dir] {
			if !slices.Contains(tolerated[dir], word) {
				out = append(out, finding{"vocabulary", where[dir+" "+word][0], fmt.Sprintf("%s says %q, a word SPEC §2.2 retired", dir, word)})
			}
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(tolerated)) {
		for _, word := range tolerated[dir] {
			if !slices.Contains(said[dir], word) {
				out = append(out, finding{"vocabulary", "specs/architecture/vocabulary_tolerated_test.go", fmt.Sprintf("%s no longer says %q: remove it from vocabularyTolerated", dir, word)})
			}
		}
	}
	return out
}

func ruleVocabularySource(w *world) []finding {
	return ruleVocabularyIn(sourceTexts(w), func(dir string) bool { return strings.HasPrefix(dir, "src/") })
}

func TestVocabularySource(t *testing.T) { judge(t, "Vocabulary") }

func TestVocabulary(t *testing.T) {
	clean, _ := worlds(t)
	for _, f := range ruleVocabulary(vocabularyTexts(t, clean)) {
		t.Errorf("%s: %s", f.where, f.what)
	}
}

func TestVocabularyBites(t *testing.T) {
	_, mutant := worlds(t)
	texts := vocabularyTexts(t, mutant)
	planted := []corpusText{
		{"docs/mutant", "docs/mutant/mutant.md:1", "The fleet of hosts"},
		{"specs/usecases/mutant", "specs/usecases/mutant/mutant.spec.yaml", "a port is leased"},
		{"apps/menubar/sources/mutant", "apps/menubar/sources/mutant/Mutant.swift:1", "Lent to linux-1"},
	}
	got := ruleVocabulary(append(texts, planted...))
	for _, want := range []string{"src/apps/mutant_vocabulary.go", "docs/mutant/mutant.md", "specs/usecases/mutant/mutant.spec.yaml", "apps/menubar/sources/mutant/Mutant.swift"} {
		if !slices.ContainsFunc(got, func(f finding) bool { return strings.HasPrefix(f.where, want) }) {
			t.Errorf("the vocabulary rule did not report %s, which says a retired word", want)
		}
	}
}

func TestVocabularyKeepsItsPhrases(t *testing.T) {
	for _, text := range []string{"a URL or host:port", "after a power cut", "the Tailnet contract", "a file's path", "Off here", "git config --global", "dclean"} {
		if got := saysBanned(codeBanned(), []string{text}); len(got) > 0 {
			t.Errorf("%q says %v, a phrase SPEC §2.2 keeps", text, got)
		}
	}
	for _, text := range []string{"lend list", "the daemon", "a probe", "Storage", "temporary VM"} {
		if got := saysBanned(codeBanned(), []string{splitText(text)}); len(got) != 1 {
			t.Errorf("%q says %v, want one retired word", text, got)
		}
	}
}

func TestVocabularyAllowlistShrinks(t *testing.T) {
	tolerated := vocabularyTolerated()
	if len(tolerated) == 0 {
		return
	}
	dir := slices.Sorted(maps.Keys(tolerated))[0]
	got := ruleVocabulary(nil)
	if !slices.ContainsFunc(got, func(f finding) bool { return strings.HasPrefix(f.what, dir+" no longer says") }) {
		t.Errorf("the vocabulary rule kept the line of %s, a folder that says nothing", dir)
	}
}
