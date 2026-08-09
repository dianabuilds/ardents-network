package architecture

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}

func walk(t *testing.T, root string, visit func(string, os.DirEntry)) {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".idea") {
			return filepath.SkipDir
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		visit(path, fileInfoEntry{FileInfo: entry})
	}
}

func relativePath(t *testing.T, root, path string) string {
	t.Helper()
	relative, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relative path for %s: %v", path, err)
	}
	return filepath.ToSlash(relative)
}

func readProjectFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return data
}

func isBuildIgnored(data []byte) bool {
	return bytes.HasPrefix(data, []byte("//go:build ignore\n"))
}

func assertPackageExports(t *testing.T, relativeDirectory string, wanted ...string) {
	t.Helper()
	directory := filepath.Join(repositoryRoot(t), filepath.FromSlash(relativeDirectory))
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if ast.IsExported(value.Name.Name) {
					got[value.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					switch item := specification.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(item.Name.Name) {
							got[item.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, name := range item.Names {
							if ast.IsExported(name.Name) {
								got[name.Name] = true
							}
						}
					}
				}
			}
		}
	}
	want := make(map[string]bool, len(wanted))
	for _, name := range wanted {
		want[name] = true
	}
	if len(got) != len(want) {
		t.Fatalf("%s exported surface = %v, want exactly %v", relativeDirectory, got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("%s Interface is missing %s", relativeDirectory, name)
		}
	}
}

type fileInfoEntry struct{ os.FileInfo }

func (entry fileInfoEntry) Type() os.FileMode          { return entry.Mode().Type() }
func (entry fileInfoEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

func Example_projectShape() {
	fmt.Println("cmd -> internal")
	// Output: cmd -> internal
}
