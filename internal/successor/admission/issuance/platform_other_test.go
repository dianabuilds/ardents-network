//go:build !linux

package issuance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedHasNoEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if e := Initialize(t.Context(), root, testBinding(1)); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e := Open(t.Context(), root, testBinding(1)); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e := os.Lstat(root); !os.IsNotExist(e) {
		t.Fatal("unsupported effect")
	}
}
