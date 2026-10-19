package architecture

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Scan every maintained Go source, including tests and build-tagged files:
// deferred QUIC cannot return through another command, platform or factory.
func TestClosedReleaseExcludesQUICImplementation(t *testing.T) {
	if err := rejectQUICImports(repositoryRoot(t)); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "consumer.go")
	for _, dependency := range []string{"github.com/quic-go/quic-go", "github.com/quic-go/quic-go/http3", modulePath + "/internal/successor/route/transport/quic"} {
		if err := os.WriteFile(path, []byte("package fixture\nimport _ "+strconv.Quote(dependency)+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := rejectQUICImports(root); err == nil {
			t.Fatalf("architecture guard accepted %s", dependency)
		}
	}
}

func rejectQUICImports(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".codex" || entry.Name() == ".agents" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			dependency, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"github.com/quic-go/quic-go", modulePath + "/internal/successor/route/transport/quic"} {
				if dependency == forbidden || strings.HasPrefix(dependency, forbidden+"/") {
					return fmt.Errorf("deferred QUIC import in %s: %s", path, dependency)
				}
			}
		}
		return nil
	})
}
