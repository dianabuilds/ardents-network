//go:build !linux

package signing

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsupportedRefusesBeforePrivateInputsAndEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "never-created")
	if e := Import(t.Context(), root, filepath.Join(root, "private.pem"), Binding{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e := Open(t.Context(), root, Binding{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e := os.Lstat(root); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("unsupported effect", e)
	}
}
