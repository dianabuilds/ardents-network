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
	for _, obsolete := range []string{"tests/qualification", "internal/qualification"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(obsolete))); !os.IsNotExist(err) {
			t.Errorf("obsolete staged qualification surface %s still exists", obsolete)
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
