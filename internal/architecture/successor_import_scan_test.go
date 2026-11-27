package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestSuccessorImportIsolation(t *testing.T) {
	checkSuccessorImports(t, "successor isolation", func(source string) bool {
		return strings.HasPrefix(source, "internal/") || strings.HasPrefix(source, "cmd/") ||
			strings.HasPrefix(source, "tests/") || strings.HasPrefix(source, "scripts/")
	})
}

// This smaller check runs independently with the Network integration profile.
func TestNetworkIntegrationImportIsolation(t *testing.T) {
	checkSuccessorImports(t, "new-domain isolation", func(source string) bool {
		return isMigratedDomain(source) || strings.HasPrefix(source, "internal/successor/") ||
			strings.HasPrefix(source, "cmd/ardents-next/") ||
			source == "scripts/sign-qualification-release.go" || offlineOperatorFile(source)
	})
}

func checkSuccessorImports(t *testing.T, scope string, includes func(string) bool) {
	t.Helper()
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !includes(relative) {
			return
		}
		// Parse every selected source, including other build tags and tests.
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !successorImportAllowed(relative, dependency) {
				t.Errorf("%s: %s imports forbidden dependency %s", scope, relative, dependency)
			}
		}
	})
}
