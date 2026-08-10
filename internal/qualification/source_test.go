package qualification

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceSHA256BindsCodeTestsAndInfrastructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, directory := range []string{".github/workflows", ".githooks", "carrier-lab", "cmd/lab", "internal/lab", "scripts"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".dockerignore": "**\n", ".github/workflows/carrier-lab.yml": "name: carrier-lab\n", ".github/workflows/quality.yml": "name: quality\n",
		".githooks/pre-commit": "make quick-check\n", "AGENTS.md": "rules\n",
		"CONTRIBUTING.md": "workflow\n", "Makefile": "check:\n", "README.md": "project\n",
		"go.mod": "module example.test/lab\n\ngo 1.26.0\n", "carrier-lab/Dockerfile": "FROM scratch\n",
		"carrier-lab/compose.yaml": "services: {}\n", "carrier-lab/tools.lock": "lock\n",
		"cmd/lab/main.go": "package main\n", "cmd/lab/main_test.go": "package main\n",
		"internal/lab/lab.go": "package lab\n", "scripts/check-tools.go": "package main\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	initial, err := SourceSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{"cmd/lab/main_test.go", "carrier-lab/compose.yaml", "Makefile"} {
		path := filepath.Join(root, filepath.FromSlash(changed))
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(original, []byte("changed\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
		observed, err := SourceSHA256(root)
		if err != nil || observed == initial {
			t.Fatalf("%s did not invalidate source identity: %v", changed, err)
		}
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
