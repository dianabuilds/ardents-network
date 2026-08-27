package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySeparatesUnitAndE2ETestsWithoutGenericLiveTree(t *testing.T) {
	root := repositoryRoot(t)
	info, err := os.Stat(filepath.Join(root, "tests", "e2e"))
	if err != nil || !info.IsDir() {
		t.Error("required test surface tests/e2e is missing")
	}
	if _, err := os.Stat(filepath.Join(root, "tests", "live")); !os.IsNotExist(err) {
		t.Error("generic tests/live tree is retired until a selected scenario owns a purpose-named boundary")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "qualification")); !os.IsNotExist(err) {
		t.Error("qualification behavior must not become a maintained internal package")
	}
	qualificationRoot := filepath.Join(root, "tests", "qualification")
	entries, err := os.ReadDir(qualificationRoot)
	if err != nil {
		t.Fatalf("read selected qualification root: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("selected qualification root has no purpose-named owner")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf("qualification root must contain purpose-named directories, found %s", entry.Name())
			continue
		}
		info, statErr := os.Stat(filepath.Join(qualificationRoot, entry.Name(), "README.md"))
		if statErr != nil || info.IsDir() {
			t.Errorf("qualification owner %s lacks README.md", entry.Name())
		}
		ubuntu, ubuntuErr := os.Stat(filepath.Join(qualificationRoot, entry.Name(), "run-ubuntu.sh"))
		windows, windowsErr := os.Stat(filepath.Join(qualificationRoot, entry.Name(), "run-windows.ps1"))
		if (ubuntuErr != nil || ubuntu.IsDir()) && (windowsErr != nil || windows.IsDir()) {
			t.Errorf("qualification owner %s lacks a purpose-named platform runner", entry.Name())
		}
	}

	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(makefile)
	for _, target := range []string{"unit:", "e2e:"} {
		if !strings.Contains(text, target) {
			t.Errorf("Makefile is missing %s", target)
		}
	}
	if strings.Contains(text, "quick-check: format-check vet test build") {
		t.Error("quick-check still runs the undifferentiated test suite")
	}
}
