package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySeparatesUnitE2EAndLiveTestsWithoutStages(t *testing.T) {
	root := repositoryRoot(t)
	for _, directory := range []string{"tests/e2e", "tests/live"} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(directory)))
		if err != nil || !info.IsDir() {
			t.Errorf("required test surface %s is missing", directory)
		}
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
	for _, target := range []string{"unit:", "e2e:", "live:"} {
		if !strings.Contains(text, target) {
			t.Errorf("Makefile is missing %s", target)
		}
	}
	if strings.Contains(text, "quick-check: format-check vet test build") {
		t.Error("quick-check still runs the undifferentiated test suite")
	}
}
