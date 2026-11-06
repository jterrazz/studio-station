package architecture_test

import (
	"fmt"
	"go/ast"
	"slices"
	"testing"
)

// invariant: rule no-sudo-literal (SC-03): station never runs sudo on a
// Mac; the word is spelled only in a home of values: the owner's hints, and
// catalog.AsRoot, which every line a VM of station's runs as root is
// composed with.
func ruleNoSudoLiteral(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.test || valuesHome(f.pkg) {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			if slices.Contains(splitWords(value), "sudo") {
				out = append(out, finding{"no-sudo-literal", f.at(lit.Pos()), fmt.Sprintf("%q spells sudo: a root line is catalog.AsRoot's, a hint surface/cli/words'", f.pkg)})
			}
		})
	}
	return out
}

func TestNoSudoLiteral(t *testing.T) { judge(t, "NoSudoLiteral") }
