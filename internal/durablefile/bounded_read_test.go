package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadBoundedReturnsExactBytesWithinBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("exact"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadBounded(path, 5, "bound refusal")
	if err != nil || string(raw) != "exact" {
		t.Fatalf("read = %q, %v", raw, err)
	}
}

func TestReadBoundedRefusesOversizedWithCallerProse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("toolong"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReadBounded(path, 3, "custom bound refusal")
	if err == nil || err.Error() != "custom bound refusal" {
		t.Fatalf("oversized read = %v; want caller bound refusal", err)
	}
}

func TestReadBoundedMissingFile(t *testing.T) {
	_, err := ReadBounded(filepath.Join(t.TempDir(), "absent"), 8, "bound")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read = %v; want not-exist", err)
	}
}
