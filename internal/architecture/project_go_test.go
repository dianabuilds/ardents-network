package architecture

import (
	"os"
	"strings"
	"testing"
)

func assertGoFilesAreProjectCode(t *testing.T, root string) {
	t.Helper()
	walk(t, root, func(path string, entry os.DirEntry) {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".go") && entry.Name() != "go.mod") {
			return
		}
		relative := relativePath(t, root, path)
		if strings.HasPrefix(relative, "experiments/") {
			if strings.HasSuffix(relative, ".go") {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("read experiment source %s: %v", relative, err)
				} else if !isBuildIgnored(data) {
					t.Errorf("Go experiment must be build-ignored so it is not a maintained root-module package: %s", relative)
				}
			}
			return
		}
		if strings.HasSuffix(relative, ".go") &&
			!strings.HasPrefix(relative, "cmd/") &&
			!strings.HasPrefix(relative, "internal/") &&
			!strings.HasPrefix(relative, "scripts/") &&
			!strings.HasPrefix(relative, "tests/") {
			t.Errorf("Go code must live in cmd, internal, scripts, or tests: %s", relative)
		}
	})
}
