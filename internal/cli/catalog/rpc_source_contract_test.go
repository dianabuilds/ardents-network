package catalog_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"ardents/internal/cli/catalog"
)

func TestProtectedCatalogueProceduresAreCalledByTheirProductionHandlers(t *testing.T) {
	for _, spec := range catalog.Commands() {
		if spec.Access != catalog.AccessProtected {
			continue
		}
		t.Run(spec.ID, func(t *testing.T) {
			directory, handler := productionHandler(t, spec.Path)
			calls := handlerCalls(t, directory, handler, spec.Path[len(spec.Path)-1])
			for _, procedure := range append([]string{spec.Procedure}, spec.SecondaryProcedures...) {
				method := procedure[strings.LastIndex(procedure, "/")+1:]
				if _, ok := calls[method]; !ok {
					t.Fatalf("handler %s does not call declared procedure %s; calls=%v", handler, procedure, calls)
				}
			}
		})
	}
}

func productionHandler(t *testing.T, path []string) (string, string) {
	t.Helper()
	group := path[0]
	directory := filepath.Join("..", group)
	file := "command.go"
	if group == "data" {
		directory = filepath.Join("..", "content")
		file = "command_data.go"
	}
	if group == "config" {
		directory = filepath.Join("..", "configuration")
	}
	handler := calledFunctionForToken(t, directory, file, "Run", path[1])
	if len(path) == 2 {
		return directory, handler
	}
	switch group + " " + path[1] {
	case "authority delivery":
		return directory, calledFunctionForToken(t, directory, "delivery.go", handler, path[2])
	case "authority rotation":
		return directory, calledFunctionForToken(t, directory, "rotation.go", handler, path[2])
	case "network resolve", "network records":
		return directory, calledFunctionForToken(t, directory, "records.go", handler, path[2])
	case "data objects":
		return directory, "dataObjects"
	case "data blobs":
		return directory, calledFunctionForToken(t, directory, "command_data_blobs.go", handler, path[2])
	case "data manifests":
		return directory, "dataManifests"
	case "data transfers":
		return directory, calledFunctionForToken(t, directory, "command_data_transfers.go", handler, path[2])
	case "identity principal", "identity device":
		return directory, calledFunctionForToken(t, directory, "command.go", handler, path[2])
	case "identity grant":
		return directory, calledFunctionForToken(t, directory, "administration.go", handler, path[2])
	case "identity delegation":
		return directory, calledFunctionForToken(t, directory, "delegation.go", handler, path[2])
	case "identity application-ticket":
		return directory, handler
	default:
		t.Fatalf("no production handler route for %q", catalog.PathString(path))
		return "", ""
	}
}

func calledFunctionForToken(t *testing.T, directory, file, function, commandToken string) string {
	t.Helper()
	functions := packageFunctions(t, directory)
	fn := functions[function]
	if fn == nil {
		t.Fatalf("%s: function %s not found", directory, function)
	}
	var called string
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || !caseHasToken(clause, commandToken) {
			return true
		}
		for _, statement := range clause.Body {
			ast.Inspect(statement, func(child ast.Node) bool {
				call, ok := child.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if ok && functions[selector.Sel.Name] != nil {
					called = selector.Sel.Name
				}
				return true
			})
		}
		return false
	})
	if called == "" {
		t.Fatalf("%s: parser %s has no handler for %q", file, function, commandToken)
	}
	return called
}

func handlerCalls(t *testing.T, directory, handler, discriminator string) map[string]struct{} {
	t.Helper()
	functions := packageFunctions(t, directory)
	result := make(map[string]struct{})
	visited := make(map[string]struct{})
	var visit func(string)
	visit = func(name string) {
		if _, ok := visited[name]; ok {
			return
		}
		visited[name] = struct{}{}
		fn := functions[name]
		if fn == nil {
			return
		}
		nodes := selectedHandlerNodes(fn, discriminator)
		for _, root := range nodes {
			ast.Inspect(root, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch target := call.Fun.(type) {
				case *ast.SelectorExpr:
					result[target.Sel.Name] = struct{}{}
					if functions[target.Sel.Name] != nil {
						visit(target.Sel.Name)
					}
				case *ast.Ident:
					if functions[target.Name] != nil {
						visit(target.Name)
					}
				}
				return true
			})
		}
	}
	visit(handler)
	return result
}

func selectedHandlerNodes(fn *ast.FuncDecl, discriminator string) []ast.Node {
	var selected []ast.Node
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CaseClause:
			if caseHasToken(value, discriminator) {
				for _, statement := range value.Body {
					selected = append(selected, statement)
				}
				return false
			}
		case *ast.KeyValueExpr:
			key, ok := value.Key.(*ast.Ident)
			if ok && key.Name == discriminator {
				selected = append(selected, value.Value)
				return false
			}
		}
		return true
	})
	if len(selected) == 0 {
		return []ast.Node{fn.Body}
	}
	return selected
}

func caseHasToken(clause *ast.CaseClause, expected string) bool {
	for _, expression := range clause.List {
		literal, ok := expression.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil && value == expected {
			return true
		}
	}
	return false
}

func packageFunctions(t *testing.T, directory string) map[string]*ast.FuncDecl {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]*ast.FuncDecl)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(files, filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if ok {
				result[fn.Name.Name] = fn
			}
		}
	}
	return result
}
