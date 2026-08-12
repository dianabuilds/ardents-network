package node

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteEvidenceFilesUsesPrivatePermissions(t *testing.T) {
	root := t.TempDir()
	observer := nodeObserver{input: Campaign{EvidenceRoot: root}}
	files := []nodeEvidenceFile{{name: "first", raw: []byte("one")}, {name: "second", raw: []byte("two")}}
	if err := observer.writeEvidenceFiles(files); err != nil {
		t.Fatal(err)
	}
	for _, expected := range files {
		raw, err := os.ReadFile(filepath.Join(root, expected.name))
		if err != nil || string(raw) != string(expected.raw) {
			t.Fatalf("evidence %s = %q, err=%v", expected.name, raw, err)
		}
		info, err := os.Stat(filepath.Join(root, expected.name))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("evidence %s permissions = %v", expected.name, info.Mode().Perm())
		}
	}
}

func TestReadNodeFixtureEvidenceSeparatesOperationAndValidationErrors(t *testing.T) {
	_, err := readNodeFixtureEvidence(t.TempDir())
	if err == nil || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "read node fixture manifest") {
		t.Fatalf("missing manifest error = %v", err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ardents-node-manifest.sha256"), []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = readNodeFixtureEvidence(root)
	if err == nil || errors.Is(err, os.ErrNotExist) || err.Error() != "node fixture manifest seal evidence is invalid" {
		t.Fatalf("invalid seal error = %v", err)
	}
}

func TestReadNodeFixtureEvidenceRejectsOversizedManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), make([]byte, (64<<10)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readNodeFixtureEvidence(root)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized manifest error = %v", err)
	}
}

func TestWriteEvidenceFilesNamesFailedArtifact(t *testing.T) {
	observer := nodeObserver{input: Campaign{EvidenceRoot: filepath.Join(t.TempDir(), "missing")}}
	err := observer.writeEvidenceFiles([]nodeEvidenceFile{{name: "manifest.json", raw: []byte("value")}})
	if err == nil || !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("write error = %v, want artifact name", err)
	}
}

func TestSourceIdentityUsesAndRevalidatesSealedSnapshot(t *testing.T) {
	repository, evidence := filepath.Join(t.TempDir(), "repository"), t.TempDir()
	compose := filepath.Join(repository, "tests", "qualification", "h3-node-v1", "compose.yaml")
	files := map[string]string{
		"go.mod": "module example.test/snapshot\n", "go.sum": "", ".dockerignore": "**\n!cmd/**\n!internal/**\n",
		"cmd/tool/main.go": "package main\n", "internal/domain/domain.go": "package domain\n",
		"tests/qualification/h3-node-v1/compose.yaml": "services: {}\n",
		"tests/qualification/h3-node-v1/Dockerfile":   "FROM scratch\n",
	}
	for name, content := range files {
		path := filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest, snapshot, err := captureNodeSourceIdentity(compose, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNodeSourceIdentity(evidence, snapshot, digest); err != nil {
		t.Fatalf("fresh source snapshot rejected: %v", err)
	}
	target := filepath.Join(snapshot, "cmd", "tool", "main.go")
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeSourceIdentity(evidence, snapshot, digest); err == nil {
		t.Fatal("mutated source snapshot passed final validation")
	}
}
