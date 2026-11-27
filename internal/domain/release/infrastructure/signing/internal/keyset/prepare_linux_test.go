package keyset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseKeySetCanceledBeforePublication(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	output := filepath.Join(t.TempDir(), "keys")
	public, err := Prepare(ctx, output)
	if err == nil || public != (Inventory{}) {
		t.Fatal("canceled preparation returned public inventory", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled preparation touched the output", err)
	}
}
