package architecture

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// pythonExceptionGroup selects a Python 3.11+ interpreter for tests whose embedded
// scripts use ExceptionGroup/BaseExceptionGroup (3.11+ builtins). The ambient
// python3 may be older (e.g. macOS system 3.9), so prefer an explicit newer
// interpreter and verify its version rather than assuming python3 suffices.
func pythonExceptionGroup(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3.13", "python3.12", "python3.11", "python3", "python"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(path, "-I", "-S", "-c", "import sys; sys.stdout.write('1' if sys.version_info >= (3, 11) else '0')").Output()
		if err == nil && strings.TrimSpace(string(out)) == "1" {
			return path
		}
	}
	t.Fatal("no Python 3.11+ interpreter available for ExceptionGroup test")
	return ""
}

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
			entry.Name() == ".codex-tmp" || entry.Name() == ".codex-remote-attachments" ||
			entry.Name() == ".claude") {
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
