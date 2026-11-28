package signing

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
	public, err := prepareReleaseKeys(ctx, output, nil)
	if err == nil || public != (releaseKeyInventory{}) {
		t.Fatal("canceled preparation returned public inventory", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled preparation touched the output", err)
	}
}
