package architecture_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const cgoFile = "src/platform/darwin/power_darwin.go"

func cgoTags() map[string]string {
	return map[string]string{
		cgoFile:                              "//go:build darwin && cgo",
		"src/platform/darwin/power_other.go": "//go:build !darwin || !cgo",
	}
}

// invariant: rule 22: cgo lives in one file, built only on darwin with cgo,
// and its refusal twin builds everywhere else, so a linux binary needs no C.
func ruleCgo(w *world) []finding {
	var out []finding
	for _, rel := range w.cgo {
		if rel != cgoFile {
			out = append(out, finding{"cgo", rel, "import \"C\" lives in " + cgoFile + " only"})
		}
	}
	for rel, tag := range cgoTags() {
		data, err := os.ReadFile(filepath.Join(w.root, rel))
		if err != nil || !strings.HasPrefix(string(data), tag+"\n") {
			out = append(out, finding{"cgo", rel, "must open with " + tag})
		}
	}
	return out
}

func unsafePower() []string {
	return []string{"disablesleep", "kIOPMAssertPreventUserIdleDisplaySleep", "kIOPMAssertionTypeNoDisplaySleep", "PreventUserIdleDisplaySleep", "NoDisplaySleep"}
}

func powerCode(rel string) bool {
	for _, dir := range []string{"src/awake/", "src/domain/power/", "src/pages/power/", "src/surface/cli/power/", "src/platform/local/power_", "src/platform/darwin/power_"} {
		if strings.HasPrefix(rel, dir) {
			return true
		}
	}
	return rel == "src/wire/power.go" || rel == "src/engine/power.go"
}

func powerPackages() []string {
	return []string{"awake", "awake/contract", "awake/adapter/macos", "domain/power"}
}

// invariant: rule 23: Power is safe by construction: no string under src/
// asks for a display assertion or disablesleep, the power code holds no
// sudo, and the awake family, its macOS adapter and domain/power reach no
// file system: no file is written for power.
func rulePowerSafe(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.test || !strings.HasPrefix(f.rel, "src/") {
			continue
		}
		ast.Inspect(f.syntax, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if n.Kind != token.STRING {
					return true
				}
				for _, word := range unsafePower() {
					if strings.Contains(n.Value, word) {
						out = append(out, finding{"power", f.at(n.Pos()), "Power never holds " + word})
					}
				}
				if powerCode(f.rel) && strings.Contains(n.Value, "sudo") {
					out = append(out, finding{"power", f.at(n.Pos()), "the power code never runs sudo"})
				}
			case *ast.Ident:
				if !slices.Contains(powerPackages(), f.pkg) {
					return true
				}
				if obj := f.info.Uses[n]; obj != nil && isFS(obj.Type()) {
					out = append(out, finding{"power", f.at(n.Pos()), f.pkg + " reaches no file system"})
				}
			}
			return true
		})
	}
	return out
}

func isFS(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Name() == "FS" && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == module+"/src/system"
}

func powerRaw(w *world) []finding {
	var out []finding
	var matches []string
	for _, dir := range []string{"local", "darwin"} {
		found, _ := filepath.Glob(filepath.Join(w.root, "src", "platform", dir, "power_*.go"))
		matches = append(matches, found...)
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		for _, word := range unsafePower() {
			if strings.Contains(string(data), word) {
				rel, _ := filepath.Rel(w.root, m)
				out = append(out, finding{"power", rel, "Power never holds " + word})
			}
		}
	}
	return out
}
