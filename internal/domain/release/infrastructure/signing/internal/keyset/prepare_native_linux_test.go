//go:build linux && network_signing_native

package keyset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReleaseKeySetNativeInterruptedCustody(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: native key publication requires root")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := filepath.Join(parent, "failed-keys")
	reached := false
	public, err := prepare(ctx, output, func(stage string, file *os.File) {
		if stage == "before-directory" {
			reached = true
			cancel()
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	})
	if !reached {
		t.Fatal("invalid environment: protected original parent required", err)
	}
	if public != (Inventory{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("failed key custody returned public inventory or lost the physical failure", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("interrupted key custody created an output directory", err)
	}
}
