//go:build linux && network_signing_native

package signing

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	mountType, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", parent).Output()
	if err != nil || strings.TrimSpace(string(mountType)) != "ext4" {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := filepath.Join(parent, "failed-keys")
	reached := false
	public, err := prepareReleaseKeys(ctx, output, func(stage string, file *os.File) {
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
	if public != (releaseKeyInventory{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("failed key custody returned public inventory or lost the physical failure", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("interrupted key custody created an output directory", err)
	}
}
