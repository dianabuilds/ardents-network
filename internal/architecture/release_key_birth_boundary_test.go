package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Key creation stays a single-purpose operation. The command is the only
// production caller, and the key producer may use only its detached inventory
// types from the surrounding signing package. The previous subpackage gave
// the latter separation through Go imports. This source check runs in CI; it
// is not a Go compiler restriction between files of the same package.
func TestReleaseKeyBirthStaysSeparateFromSigning(t *testing.T) {
	directory := filepath.Join(repositoryRoot(t), "internal/domain/release/infrastructure/signing")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	otherNames := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = file
		if entry.Name() == "release_key_birth_linux.go" {
			continue
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					otherNames[declaration.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						otherNames[spec.Name.Name] = true
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							otherNames[name.Name] = true
						}
					}
				}
			}
		}
	}
	birth := files["release_key_birth_linux.go"]
	if birth == nil {
		t.Fatal("release key producer is absent")
	}
	ast.Inspect(birth, func(node ast.Node) bool {
		name, ok := node.(*ast.Ident)
		if ok && name.Obj == nil && otherNames[name.Name] && name.Name != "releaseKeyInventory" && name.Name != "releaseKeySlot" {
			t.Errorf("key producer refers to unrelated signing name %s", name.Name)
		}
		return true
	})
	for filename, file := range files {
		if filename == "release_key_birth_linux.go" || filename == "release_key_command_linux.go" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if name, ok := node.(*ast.Ident); ok && name.Name == "prepareReleaseKeys" {
				t.Errorf("%s uses key creation outside its command", filename)
			}
			return true
		})
	}
}
