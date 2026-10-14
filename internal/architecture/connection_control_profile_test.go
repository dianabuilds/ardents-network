package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// The native scheduler selects source callsites, not permission or record
// substitutes. A source move must fail qualification rather than silently
// change which actual I/O boundary the fixture interrupts.
func TestSuccessorConnectionControlProfileSelectsOriginalReadSites(t *testing.T) {
	root := repositoryRoot(t)
	set := token.NewFileSet()
	parse := func(path string) *ast.File {
		t.Helper()
		file, err := parser.ParseFile(set, filepath.Join(root, path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	fixture := parse("cmd/ardents-next/connection_response_loss_linux_test.go")
	constants := make(map[string]int)
	ast.Inspect(fixture, func(node ast.Node) bool {
		value, ok := node.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
			return true
		}
		literal, ok := value.Values[0].(*ast.BasicLit)
		if ok && literal.Kind == token.INT {
			line, err := strconv.Atoi(literal.Value)
			if err == nil {
				constants[value.Names[0].Name] = line
			}
		}
		return true
	})
	for _, site := range []struct {
		path, constant string
		postRead       bool
	}{
		{"internal/successor/connection/attachment_record_reader.go", "connectionControlFramingReadLine", false},
		{"internal/successor/publication/instance/service_attachment.go", "connectionControlReadRecheckLine", true},
	} {
		file := parse(site.path)
		matches := 0
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "Read" || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				position := token.NoPos
				if !site.postRead {
					call, ok := node.(*ast.CallExpr)
					if !ok || len(call.Args) != 1 {
						return true
					}
					name, ok := call.Fun.(*ast.SelectorExpr)
					body, isBody := call.Args[0].(*ast.SliceExpr)
					if ok && name.Sel.Name == "Read" && isBody && body.Low == nil && body.High == nil && body.Max == nil {
						buffer, isBuffer := body.X.(*ast.SelectorExpr)
						reader, isReader := name.X.(*ast.SelectorExpr)
						if isBuffer && isReader && buffer.Sel.Name == "buffer" && reader.Sel.Name == "reader" {
							position = call.Pos()
						}
					}
				} else if returned, ok := node.(*ast.ReturnStmt); ok {
					ast.Inspect(returned, func(child ast.Node) bool {
						call, ok := child.(*ast.CallExpr)
						if !ok {
							return true
						}
						name, ok := call.Fun.(*ast.SelectorExpr)
						if ok && name.Sel.Name == "recheck" {
							position = returned.Pos()
						}
						return true
					})
				}
				if position != token.NoPos {
					matches++
					if line := set.Position(position).Line; constants[site.constant] != line {
						t.Errorf("%s selects line %d, original %s boundary is %d", site.constant, constants[site.constant], site.path, line)
					}
				}
				return true
			})
		}
		if matches != 1 {
			t.Errorf("%s has %d selected original read sites", site.path, matches)
		}
	}
}

func TestSuccessorConnectionControlProfileSelectsOriginalWriteSites(t *testing.T) {
	root := repositoryRoot(t)
	set := token.NewFileSet()
	fixture, err := parser.ParseFile(set, filepath.Join(root, "cmd/ardents-next/connection_response_loss_linux_test.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	constants := make(map[string]int)
	ast.Inspect(fixture, func(node ast.Node) bool {
		value, ok := node.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
			return true
		}
		literal, ok := value.Values[0].(*ast.BasicLit)
		if ok && literal.Kind == token.INT {
			line, err := strconv.Atoi(literal.Value)
			if err == nil {
				constants[value.Names[0].Name] = line
			}
		}
		return true
	})
	for _, site := range []struct {
		path, receiver, constant, check string
		after                           bool
	}{
		{"internal/successor/publication/instance/service_attachment.go", "ServiceAttachment", "connectionPublisherControlWriteBeforeLine", "enter", false},
		{"internal/successor/publication/instance/service_attachment.go", "ServiceAttachment", "connectionPublisherControlWriteAfterLine", "recheck", true},
		{"internal/successor/connection/initiator_attachment.go", "initiatorAttachment", "connectionInitiatorControlWriteBeforeLine", "enter", false},
		{"internal/successor/connection/initiator_attachment.go", "initiatorAttachment", "connectionInitiatorControlWriteAfterLine", "check", true},
	} {
		file, err := parser.ParseFile(set, filepath.Join(root, site.path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		matches := 0
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "Write" || function.Recv == nil || function.Body == nil {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			name, ok := pointer.X.(*ast.Ident)
			if !ok || name.Name != site.receiver {
				continue
			}
			for _, statement := range function.Body.List {
				var position token.Pos
				ast.Inspect(statement, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if ok && selector.Sel.Name == site.check {
						if site.after {
							position = statement.Pos()
						} else {
							position = call.Pos()
						}
					}
					return true
				})
				if position != token.NoPos {
					matches++
					if line := set.Position(position).Line; constants[site.constant] != line {
						t.Errorf("%s selects line %d, original %s boundary is %d", site.constant, constants[site.constant], site.path, line)
					}
				}
			}
		}
		if matches != 1 {
			t.Errorf("%s has %d selected original write sites", site.constant, matches)
		}
	}
}
