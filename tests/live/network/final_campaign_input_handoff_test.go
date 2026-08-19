//go:build live

package network_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFinalWorkerInputCopyRequiresFrozenStableHash(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	raw := []byte("frozen worker input\n")
	if err := os.WriteFile(source, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	expected := hex.EncodeToString(digest[:])
	if err := copyFinalWorkerInput(source, filepath.Join(root, "copy"), expected); err != nil {
		t.Fatal(err)
	}
	if err := copyFinalWorkerInput(source, filepath.Join(root, "wrong-copy"),
		hex.EncodeToString(make([]byte, 32))); err == nil {
		t.Fatal("worker input with a changed frozen hash was accepted")
	}
}

func TestFinalWorkerInputCopyRejectsSymlinkAndOversize(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("input\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(source, link); err == nil {
		if err := copyFinalWorkerInput(link, filepath.Join(root, "link-copy"),
			hex.EncodeToString(make([]byte, 32))); err == nil {
			t.Fatal("symlinked worker input was accepted")
		}
	}
	oversized := filepath.Join(root, "oversized")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(file.Truncate(maximumFinalWorkerInput+1), file.Close()); err != nil {
		t.Fatal(err)
	}
	if err := copyFinalWorkerInput(oversized, filepath.Join(root, "oversized-copy"),
		hex.EncodeToString(make([]byte, 32))); err == nil {
		t.Fatal("oversized worker input was accepted")
	}
}
