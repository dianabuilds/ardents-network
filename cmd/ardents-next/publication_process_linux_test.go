package main

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPublicationCompiledRootPreparation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "independent-publication")
	target := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	network := hex.EncodeToString(bytes.Repeat([]byte{2}, 32))
	command := compiledCommand(t)
	run := func(expected string, success bool) {
		t.Helper()
		out, err := exec.Command(command, "publication", "prepare-root", root, target, network).CombinedOutput()
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
