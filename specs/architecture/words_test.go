package architecture_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// invariant: rule technology-words, one row per adapter: the words a
// technology is spelled with, which only adapter/<that adapter> may say.
func technologies() map[string][]string {
	return map[string][]string{
		"tart":        {"tart"},
		"ssh":         {"ssh"},
		"sshstdio":    {"ssh"},
		"openssh":     {"ssh"},
		"tailscale":   {"tailscale", "tailscaled"},
		"funnel":      {"tailscale"},
		"cloudflare":  {"cloudflared"},
		"launchd":     {"launchd", "launchctl"},
		"brew":        {"brew"},
		"mas":         {"mas"},
		"mise":        {"mise"},
		"npm":         {"npm"},
		"bun":         {"bun"},
		"uv":          {"uv"},
		"docker":      {"docker"},
		"cli":         {"docker", "npm"},
		"lsof":        {"lsof"},
		"keychain":    {"security"},
		"neversleeps": {"pmset"},
		"macos":       {"pmset", "sysctl", "security", "fdesetup", "ssh"},
		"linux":       {},
	}
}

func technologyWords() []string {
	var out []string
	for _, words := range technologies() {
		for _, w := range words {
			if !slices.Contains(out, w) {
				out = append(out, w)
			}
		}
	}
	return append(out, "scp")
}

// invariant: the allowlist of technology-words: today's packages and the
// words they still say. It only shrinks; a line no longer true fails the
// gate until it is removed.
func wordsTolerated() map[string][]string {
	return map[string][]string{
		"wire":                         {"bun", "mise", "ssh"},
		"surface/cli/kit":              {"docker", "fdesetup", "ssh"},
		"surface/cli/machines":         {"ssh"},
		"surface/cli/machines/restart": {"fdesetup"},
		"surface/cli/run":              {"docker"},
		"surface/cli/run/docker":       {"docker"},
		"surface/cli/tailnet":          {"tailscale"},
		"cmd/generate/pages":           {"brew", "bun", "docker", "launchctl", "mas", "mise", "npm", "security", "ssh", "tailscale", "uv"},
		"domain/accounts":              {"docker", "npm", "ssh", "tailscale"},
		"domain/config":                {"fdesetup", "pmset", "security", "ssh"},
		"engine":                       {"ssh"},
		"domain/guest":                 {"docker"},
		"domain/machines":              {"ssh", "sysctl"},
		"domain/remote":                {"ssh"},
		"domain/setup":                 {"bun", "mise", "npm", "tailscale", "tailscaled"},
		"domain/shell":                 {"mise"},
		"domain/shortcuts":             {"docker"},
		"domain/status":                {"brew", "launchd", "pmset", "security", "sysctl", "tailscale"},
		"domain/storage":               {"brew", "docker", "mise"},
		"pages/config":                 {"security"},
		"pages/home":                   {"brew", "docker", "mise", "npm", "security", "tailscale"},
		"pages/machines":               {"ssh"},
		"pages/power":                  {"pmset"},
		"pages/services":               {"ssh"},
		"pages/shortcuts":              {"docker"},
		"pages/storage":                {"brew"},
		"pages/tailnet":                {"tailscale"},
	}
}

func splitWords(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if i > 0 && unicode.IsUpper(r) && len(cur) > 0 {
			prev := rs[i-1]
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return out
}

func wordAllowed(pkg, word string) bool {
	if pkg == "catalog" || strings.HasPrefix(pkg, "catalog/") || pkg == "surface/cli/words" {
		return true
	}
	parts := strings.Split(pkg, "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "adapter" && slices.Contains(technologies()[parts[i+1]], word) {
			return true
		}
	}
	return false
}

// invariant: ssh is also a product word (the SSH section, machines.ssh,
// SSHConfig): its identifiers are exempt, an ssh command line is not, and a
// literal that runs ssh is one: an exec's name or path, an argv's first
// word, a shell line that calls it.
func sshCommandLine(fields []string) bool {
	for i, f := range fields {
		if path.Base(f) != "ssh" {
			continue
		}
		switch {
		case i == 0, strings.Contains(f, "/"):
			return true
		case slices.Contains([]string{"exec", "sudo", "env", "command", "&&", "||", "|", ";"}, fields[i-1]):
			return true
		}
	}
	return false
}

func literalWords(value string) []string {
	words := splitWords(value)
	if !sshCommandLine(strings.Fields(value)) {
		words = slices.DeleteFunc(words, func(w string) bool { return w == "ssh" })
	}
	return words
}

func identifierWords(name string) []string {
	return slices.DeleteFunc(splitWords(name), func(w string) bool { return w == "ssh" })
}

// invariant: the value of a constant of a named type is a product name, like
// the SSH section's ID: it is exempt from ssh as an identifier is.
func namedConstants(f *file) map[ast.Node]bool {
	named := map[ast.Node]bool{}
	ast.Inspect(f.syntax, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != len(spec.Values) {
			return true
		}
		for i, name := range spec.Names {
			c, ok := f.info.Defs[name].(*types.Const)
			if !ok {
				continue
			}
			if _, isNamed := types.Unalias(c.Type()).(*types.Named); isNamed {
				named[spec.Values[i]] = true
			}
		}
		return true
	})
	return named
}

func adapterPackage(obj types.Object) bool {
	pn, ok := obj.(*types.PkgName)
	return ok && isAdapter(strings.TrimPrefix(pn.Imported().Path(), module+"/src/"))
}

type said struct{ pkg, word string }

func technologySaid(w *world) (map[said][]string, []finding) {
	vocabulary := technologyWords()
	seen := map[said][]string{}
	var out []finding
	report := func(f *file, at, word string) {
		if !slices.Contains(vocabulary, word) || wordAllowed(f.pkg, word) {
			return
		}
		key := said{f.pkg, word}
		seen[key] = append(seen[key], at)
		if !slices.Contains(wordsTolerated()[f.pkg], word) {
			out = append(out, finding{"technology-words", at, fmt.Sprintf("%q names the technology %q: only its adapter may", f.pkg, word)})
		}
	}
	for _, f := range w.files {
		imports := map[ast.Node]bool{}
		for _, spec := range f.syntax.Imports {
			imports[spec.Path] = true
			if spec.Name != nil {
				imports[spec.Name] = true
			}
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			if imports[n] {
				return false
			}
			x, ok := n.(*ast.Ident)
			if !ok || x == f.syntax.Name {
				return true
			}
			if obj := f.info.Uses[x]; obj != nil && adapterPackage(obj) {
				return true
			}
			for _, word := range identifierWords(x.Name) {
				report(f, f.at(x.Pos()), word)
			}
			return true
		})
		named := namedConstants(f)
		literals(f, func(lit *ast.BasicLit, value string) {
			if imports[lit] {
				return
			}
			words := literalWords(value)
			if named[lit] {
				words = identifierWords(value)
			}
			for _, word := range words {
				report(f, f.at(lit.Pos()), word)
			}
		})
	}
	return seen, out
}

func ruleTechnologyWords(w *world) []finding {
	seen, out := technologySaid(w)
	for _, pkg := range slices.Sorted(maps.Keys(wordsTolerated())) {
		for _, word := range wordsTolerated()[pkg] {
			if len(seen[said{pkg, word}]) == 0 {
				out = append(out, finding{"technology-words", "specs/architecture/words_test.go", fmt.Sprintf("src/%s no longer says %q: remove it from wordsTolerated", pkg, word)})
			}
		}
	}
	return out
}

func TestTechnologyWords(t *testing.T)        { judge(t, "TechnologyWords") }
func TestTechnologyWordsSSH(t *testing.T)     { judge(t, "TechnologyWordsSSH") }
func TestTechnologyWordsSSHPath(t *testing.T) { judge(t, "TechnologyWordsSSHPath") }
func TestTechnologyWordsSSHLine(t *testing.T) { judge(t, "TechnologyWordsSSHLine") }

func TestTechnologyWordsAllowlistShrinks(t *testing.T) {
	clean, _ := worlds(t)
	pkg := slices.Sorted(maps.Keys(wordsTolerated()))[0]
	got := ruleTechnologyWords(withoutPackage(clean, func(p string) bool { return p == pkg }))
	if !slices.ContainsFunc(got, func(f finding) bool { return strings.HasPrefix(f.what, "src/"+pkg+" no longer says") }) {
		t.Errorf("the technology-words rule kept the line of src/%s, a package that is gone", pkg)
	}
}
