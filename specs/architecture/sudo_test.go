package architecture_test

import (
	"fmt"
	"go/ast"
	"slices"
	"testing"
)

// invariant: the allowlist of no-sudo-literal: today's packages that still
// spell sudo. Most are a hint for the owner; engine, images/calls and
// domain/machines run sudo inside a VM; machines/adapter/macos runs
// `sudo fdesetup authrestart` on another Mac, the planned restart of SPEC
// §5.13, which security/never-sudo does not cover. It only shrinks.
func sudoTolerated() []string {
	return []string{
		"background/adapter/launchd", "cmd/generate/pages", "images/calls",
		"domain/machines", "engine", "machines/adapter/macos", "machines/contract/contracttest",
		"surface/cli/kit", "wire",
	}
}

// invariant: rule no-sudo-literal (SC-03): station never runs sudo; the word
// is spelled only in surface/cli/words, the hints that tell the owner to.
func ruleNoSudoLiteral(w *world) []finding {
	var out []finding
	seen := map[string]bool{}
	for _, f := range w.files {
		if f.test || f.pkg == "surface/cli/words" {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			if !slices.Contains(splitWords(value), "sudo") {
				return
			}
			seen[f.pkg] = true
			if !slices.Contains(sudoTolerated(), f.pkg) {
				out = append(out, finding{"no-sudo-literal", f.at(lit.Pos()), fmt.Sprintf("%q spells sudo: station never runs it, and only surface/cli/words hints it", f.pkg)})
			}
		})
	}
	for _, pkg := range sudoTolerated() {
		if !seen[pkg] {
			out = append(out, finding{"no-sudo-literal", "specs/architecture/sudo_test.go", fmt.Sprintf("src/%s no longer spells sudo: remove it from sudoTolerated", pkg)})
		}
	}
	return out
}

func TestNoSudoLiteral(t *testing.T) { judge(t, "NoSudoLiteral") }
