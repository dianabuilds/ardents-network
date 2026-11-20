package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The Stream worker inventory is admitted only by the trusted qualification
// launch path (#654 3c-A). Ordinary text jobs use Activate/ObserveInstance;
// no ordinary runtime, command or Application file may reach the Stream
// entries, and the worker package exposes no caller-visible inventory
// selector. The exact caller allowlist grows only when the accepted
// qualification runner lands; a new caller is a boundary change, not a
// routine edit.
func TestStreamWorkerActivationHasOnlyTrustedCallers(t *testing.T) {
	root := repositoryRoot(t)
	workerPackage := modulePath + "/internal/successor/execution/worker"
	allowed := map[string]bool{
		"internal/successor/execution/worker/stream_activation_installed_linux_test.go": true,
	}
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !strings.HasPrefix(relative, "internal/") && !strings.HasPrefix(relative, "cmd/") && !strings.HasPrefix(relative, "tests/") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		inWorkerPackage := strings.HasPrefix(relative, "internal/successor/execution/worker/")
		importsWorker := false
		for _, imported := range file.Imports {
			if dependency, err := strconv.Unquote(imported.Path.Value); err == nil && dependency == workerPackage {
				importsWorker = true
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if !inWorkerPackage {
					return true
				}
				name = fun.Name
			case *ast.SelectorExpr:
				if !importsWorker {
					return true
				}
				name = fun.Sel.Name
			default:
				return true
			}
			if name != "ActivateStream" && name != "ObserveInstanceStream" {
				return true
			}
			if !allowed[relative] {
				t.Errorf("stream worker boundary: %s calls %s outside the trusted qualification launch path", relative, name)
			}
			return true
		})
	})
}
