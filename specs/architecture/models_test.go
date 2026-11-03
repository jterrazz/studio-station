package architecture_test

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"
	"strings"
	"testing"
)

func modelNames() []string {
	return []string{"MacBook", "Macmini", "Mac mini", "iMac", "Mac Studio", "MacPro", "Mac Pro"}
}

// invariant: the allowlist of no-model-names: today's packages and the model
// names they still spell. It only shrinks.
func modelsTolerated() map[string][]string {
	return map[string][]string{}
}

func modelNameAllowed(pkg string) bool {
	return pkg == "catalog" || strings.HasPrefix(pkg, "catalog/") || pkg == "surface/cli/words" || slices.Contains(presentation(), topOf(pkg))
}

// invariant: rule no-model-names (E1): a rule reads the capabilities a Mac
// reports, never its model: a model name is spelled only by the catalogue
// and the presentation.
func ruleNoModelNames(w *world) []finding {
	var out []finding
	seen := map[said]bool{}
	for _, f := range w.files {
		if modelNameAllowed(f.pkg) {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			for _, name := range modelNames() {
				if !strings.Contains(value, name) {
					continue
				}
				seen[said{f.pkg, name}] = true
				if !slices.Contains(modelsTolerated()[f.pkg], name) {
					out = append(out, finding{"no-model-names", f.at(lit.Pos()), fmt.Sprintf("%q spells the model name %q: a rule reads capabilities", f.pkg, name)})
				}
			}
		})
	}
	for _, pkg := range slices.Sorted(maps.Keys(modelsTolerated())) {
		for _, name := range modelsTolerated()[pkg] {
			if !seen[said{pkg, name}] {
				out = append(out, finding{"no-model-names", "specs/architecture/models_test.go", fmt.Sprintf("src/%s no longer spells %q: remove it from modelsTolerated", pkg, name)})
			}
		}
	}
	return out
}

func TestNoModelNames(t *testing.T) { judge(t, "NoModelNames") }
