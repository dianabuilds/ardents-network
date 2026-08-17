package camouflage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

func TestServeRejectsUnpinnedBinaryBeforeMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binaryPath := filepath.Join(root, "candidate")
	if err := os.WriteFile(binaryPath, []byte("not the pinned WebTunnel server"), 0o700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "state")
	config, err := camouflage.Validate(candidateEnvelope(), [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	serving, err := camouflage.Serve(context.Background(), config, camouflage.Server{
		Binary: binaryPath, StateRoot: stateRoot, Deadline: time.Now().Add(time.Second), ResourceProfile: "h3-s-v1",
	})
	if err == nil || err.Error() != "adapter-supply-invalid" {
		t.Fatalf("Serve() error = %v, want adapter-supply-invalid", err)
	}
	if serving != nil {
		t.Fatal("rejected supply returned a server owner")
	}
	if _, statErr := os.Stat(stateRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected supply mutated state root: %v", statErr)
	}
}
