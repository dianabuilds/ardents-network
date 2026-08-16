package blockedentry

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRejectsEvidenceInsideWorkspaceBeforeMutation(t *testing.T) {
	workspace := t.TempDir()
	inside := filepath.Join(workspace, "evidence")
	_, err := Run(Config{WorkspaceRoot: workspace, EvidenceRoot: inside, RunID: "run",
		VerifierPath: os.Args[0], ClientPath: os.Args[0], ServerPath: os.Args[0]})
	if err == nil {
		t.Fatal("repository-local evidence root was accepted")
	}
	if _, statErr := os.Stat(inside); !os.IsNotExist(statErr) {
		t.Fatalf("rejected evidence root was mutated: %v", statErr)
	}
}

func TestFreezeSupplyExecutesOwnedSnapshot(t *testing.T) {
	source := filepath.Join(t.TempDir(), "runner.exe")
	if err := os.WriteFile(source, []byte("first-binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.Mkdir(secret, 0o700); err != nil {
		t.Fatal(err)
	}
	config, err := freezeSupply(Config{RunnerPath: source, ClientPath: source, ServerPath: source}, secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("replacement"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(config.RunnerPath)
	if err != nil || !bytes.Equal(raw, []byte("first-binary")) {
		t.Fatalf("frozen runner changed with caller path: %q %v", raw, err)
	}
}
