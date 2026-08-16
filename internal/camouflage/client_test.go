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

func TestOpenClientRejectsUnpinnedBinaryBeforeMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binaryPath := filepath.Join(root, "candidate")
	if err := os.WriteFile(binaryPath, []byte("not the pinned WebTunnel client"), 0o700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "state")
	config, err := camouflage.Validate(candidateEnvelope(), [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	carrier, cleanup, err := camouflage.OpenClient(context.Background(), config, camouflage.Client{
		Binary: binaryPath, StateRoot: stateRoot, Deadline: time.Now().Add(time.Second),
	})
	if err == nil || err.Error() != "adapter-supply-invalid" {
		t.Fatalf("OpenClient() error = %v, want adapter-supply-invalid", err)
	}
	if carrier != nil || cleanup != nil {
		t.Fatal("rejected supply returned a carrier or cleanup owner")
	}
	if _, statErr := os.Stat(stateRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected supply mutated state root: %v", statErr)
	}
}
