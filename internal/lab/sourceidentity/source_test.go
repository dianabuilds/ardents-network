package sourceidentity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceSHA256BindsBothExperimentInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, directory := range []string{".github/workflows", ".githooks", "cmd/carrier-lab", "cmd/named-site-lab", "internal/lab/sourceidentity", "lab/carrier", "lab/named-site", "scripts"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".dockerignore": "**\n", ".github/workflows/carrier-lab.yml": "name: carrier-lab\n",
		".github/workflows/gate-c.yml": "name: gate-c\n", ".github/workflows/quality.yml": "name: quality\n",
		".githooks/pre-commit": "make quick-check\n", "AGENTS.md": "rules\n", "CONTRIBUTING.md": "workflow\n",
		"Makefile": "check:\n", "README.md": "project\n", "go.mod": "module example.test/lab\n\ngo 1.26.0\n", "go.sum": "example.test/dependency v1.0.0 h1:test=\n",
		"lab/README.md": "laboratory documentation\n", "lab/carrier/Dockerfile": "FROM scratch\n", "lab/carrier/compose.yaml": "services: {}\n",
		"lab/named-site/Dockerfile": "FROM scratch\n", "lab/named-site/compose.yaml": "services: {}\n",
		"cmd/carrier-lab/main.go": "package main\n", "internal/lab/sourceidentity/source.go": "package sourceidentity\n", "scripts/check-tools.go": "package main\n",
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
	for _, changed := range []string{"lab/carrier/compose.yaml", "lab/named-site/Dockerfile", ".github/workflows/gate-c.yml", "go.sum"} {
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
	if err := os.WriteFile(filepath.Join(root, "lab", "README.md"), []byte("updated laboratory documentation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err := SourceSHA256(root)
	if err != nil || observed != initial {
		t.Fatalf("human-facing terminal summary changed experiment identity: %v", err)
	}
}
