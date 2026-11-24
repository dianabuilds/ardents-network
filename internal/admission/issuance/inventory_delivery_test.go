package issuance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeliverInventoryRefusesOwnedDestinationBeforeInitialization(t *testing.T) {
	root := filepath.Join(t.TempDir(), "issuer")
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	binding := Binding{
		Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte{3},
		Start: start, End: start.Add(time.Hour),
	}

	phase, err := DeliverInventory(t.Context(), root, binding, filepath.Join(root, "inventory.json"), true)
	if phase != "input" || !errors.Is(err, ErrInvalid) {
		t.Fatalf("owned destination: phase %q, error %v", phase, err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused delivery created its key root: %v", err)
	}
}
