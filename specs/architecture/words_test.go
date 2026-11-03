package architecture_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
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
		"app":                  {"bun", "launchctl", "launchd", "mise", "pmset", "tart"},
		"cli/kit":              {"docker", "fdesetup", "launchctl", "ssh", "tailscale", "tart"},
		"cli/machines":         {"ssh"},
		"cli/machines/restart": {"fdesetup"},
		"cli/run":              {"docker"},
		"cli/run/docker":       {"docker"},
		"cli/tailnet":          {"tailscale"},
		"cmd/generate/pages":   {"brew", "bun", "docker", "launchctl", "mas", "mise", "npm", "security", "ssh", "tailscale", "tart", "uv"},
		"domain/accounts":      {"docker", "npm", "tailscale"},
		"domain/config":        {"fdesetup", "launchctl", "pmset", "security", "ssh"},
		"domain/daemon":        {"launchctl", "pmset", "ssh", "tailscale", "tart"},
		"domain/guest":         {"docker", "lsof"},
		"domain/images":        {"scp", "tart"},
		"domain/machines":      {"fdesetup", "launchctl", "ssh", "sysctl", "tart"},
		"domain/power":         {"pmset"},
		"domain/probe":         {"brew", "docker"},
		"domain/settings":      {"security", "sysctl"},
		"domain/setup":         {"bun", "mise", "npm", "tailscale", "tailscaled"},
		"domain/shell":         {"mise"},
		"domain/shortcuts":     {"docker"},
		"domain/status":        {"brew", "bun", "docker", "fdesetup", "launchctl", "launchd", "lsof", "npm", "pmset", "security", "sysctl", "tailscale", "tart"},
		"domain/storage":       {"brew", "docker", "mise"},
		"domain/tailnet":       {"tailscale"},
		"domain/tools":         {"brew", "bun", "mas", "mise", "npm", "uv"},
		"pages/config":         {"security"},
		"pages/home":           {"brew", "docker", "mise", "npm", "security", "tailscale"},
		"pages/power":          {"pmset"},
		"pages/shortcuts":      {"docker"},
		"pages/storage":        {"brew"},
		"pages/tailnet":        {"tailscale"},
		"system/tart":          {"tart"},
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
// SSHConfig): its identifiers are exempt, an ssh command line is not.
func literalWords(value string) []string {
	words := splitWords(value)
	if fields := strings.Fields(value); len(fields) < 2 || fields[0] != "ssh" {
		words = slices.DeleteFunc(words, func(w string) bool { return w == "ssh" })
	}
	return words
}

func identifierWords(name string) []string {
	return slices.DeleteFunc(splitWords(name), func(w string) bool { return w == "ssh" })
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
		literals(f, func(lit *ast.BasicLit, value string) {
			if imports[lit] {
				return
			}
			for _, word := range literalWords(value) {
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

func TestTechnologyWords(t *testing.T) { judge(t, "TechnologyWords") }
