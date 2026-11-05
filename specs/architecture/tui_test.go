package architecture_test

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"
)

// invariant: rule tui-loading-law (TU-24): a view never reads. The TUI's one
// read, its Pages, and any use-case call (core/call, a family's calls) are
// called only inside a function literal, the command Bubble Tea runs off the
// frame; a method that calls one draws by reading.
func ruleTUILoading(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.test || !under(f.pkg, "surface/tui", "surface/tui/*") {
			continue
		}
		var walk func(n ast.Node, inCmd bool)
		walk = func(n ast.Node, inCmd bool) {
			ast.Inspect(n, func(x ast.Node) bool {
				switch x := x.(type) {
				case *ast.FuncLit:
					if x != n {
						walk(x, true)
						return false
					}
				case *ast.CallExpr:
					if !inCmd && (isPages(f.info.TypeOf(x.Fun)) || isUseCase(f.info, x.Fun)) {
						out = append(out, finding{"tui-loading-law", f.at(x.Pos()), "the TUI reads outside a command: a view would wait on it"})
					}
				}
				return true
			})
		}
		walk(f.syntax, false)
	}
	return out
}

func isPages(t types.Type) bool {
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == module+"/src/surface/tui" && named.Obj().Name() == "Pages"
}

func isUseCase(info *types.Info, fun ast.Expr) bool {
	var id *ast.Ident
	switch e := fun.(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	case *ast.IndexExpr:
		return isUseCase(info, e.X)
	case *ast.IndexListExpr:
		return isUseCase(info, e.X)
	default:
		return false
	}
	fn, ok := info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	return path == module+"/src/core/call" || (strings.HasPrefix(path, module+"/src/") && strings.HasSuffix(path, "/calls"))
}

func TestTUILoadingLaw(t *testing.T)      { judge(t, "TUILoadingLaw") }
func TestTUILoadingLawCalls(t *testing.T) { judge(t, "TUILoadingLawCalls") }
