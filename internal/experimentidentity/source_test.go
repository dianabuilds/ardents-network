package experimentidentity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceSHA256BindsBothExperimentInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, directory := range []string{".github/workflows", ".githooks", "carrier-lab", "reference-site", "cmd/lab", "internal/lab", "scripts"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".dockerignore": "**\n", ".github/workflows/carrier-lab.yml": "name: carrier-lab\n",
		".github/workflows/gate-c.yml": "name: gate-c\n", ".github/workflows/quality.yml": "name: quality\n",
		".githooks/pre-commit": "make quick-check\n", "AGENTS.md": "rules\n", "CONTRIBUTING.md": "workflow\n",
		"Makefile": "check:\n", "README.md": "project\n", "go.mod": "module example.test/lab\n\ngo 1.26.0\n", "go.sum": "example.test/dependency v1.0.0 h1:test=\n",
		"carrier-lab/Dockerfile": "FROM scratch\n", "carrier-lab/compose.yaml": "services: {}\n",
		"reference-site/Dockerfile": "FROM scratch\n", "reference-site/compose.yaml": "services: {}\n",
		"cmd/lab/main.go": "package main\n", "internal/lab/lab.go": "package lab\n", "scripts/check-tools.go": "package main\n",
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
	for _, changed := range []string{"carrier-lab/compose.yaml", "reference-site/Dockerfile", ".github/workflows/gate-c.yml", "go.sum"} {
		path := filepath.Join(root, filepath.FromSlash(changed))
		if err := os.WriteFile(path, []byte("changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		observed, err := SourceSHA256(root)
		if err != nil || observed == initial {
			t.Fatalf("%s did not invalidate source identity: %v", changed, err)
		}
		if err := os.WriteFile(path, []byte(files[changed]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("terminal result summary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err := SourceSHA256(root)
	if err != nil || observed != initial {
		t.Fatalf("human-facing terminal summary changed experiment identity: %v", err)
	}
}
