//go:build linux || darwin || windows

// prepare-root opens the durable publication lease implemented on the Unix
// family (durable lease_unix.go: Linux and macOS) and Windows
// (lease_windows.go). macOS locates the Go temp directory under the system
// /var symlink, which publication root preparation refuses, so the fixture
// binds the root to the same physical directory by canonical path.
package service_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPublicationCompiledRootPreparation(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "independent-publication")
	target := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	network := hex.EncodeToString(bytes.Repeat([]byte{2}, 32))
	command := buildProductCommand(t, "ardents-next")
	run := func(expected string, success bool) {
		t.Helper()
		out, err := exec.CommandContext(t.Context(), command, "publication", "prepare-root", root, target, network).CombinedOutput()
		if (err == nil) != success || string(out) != expected {
			t.Fatalf("consumer: %s, %v", out, err)
		}
	}
	run("{\"outcome\":\"publication-root-prepared\",\"floor\":0}\n", true)
	if err := os.WriteFile(filepath.Join(root, "floor"), []byte("18446744073709551615\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("{\"outcome\":\"publication-root-prepared\",\"floor\":18446744073709551615}\n", true)
	if err := os.WriteFile(filepath.Join(root, "floor"), []byte("18446744073709551616\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("{\"outcome\":\"publication-unavailable\"}\n", false)
	kept, err := os.ReadFile(filepath.Join(root, "floor"))
	if err != nil || string(kept) != "18446744073709551616\n" {
		t.Fatal("refusal changed floor", err)
	}
}
