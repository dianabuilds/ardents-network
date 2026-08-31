package architecture

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".idea" ||
			entry.Name() == ".codex-tmp" || entry.Name() == ".codex-remote-attachments") {
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

type fileInfoEntry struct{ os.FileInfo }

func (entry fileInfoEntry) Type() os.FileMode          { return entry.Mode().Type() }
func (entry fileInfoEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

func Example_projectShape() {
	fmt.Println("cmd -> internal")
	// Output: cmd -> internal
}
