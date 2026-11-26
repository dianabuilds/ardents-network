//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package enrollment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Unix mode/link policy has no equivalent portable Windows assertion. These
// tests justify their native profile; byte and canonical tests stay portable.
func TestUnixOwnershipAndLinkedInventoryRefuse(t *testing.T) {
	for _, scenario := range []string{"mode", "hardlink", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			r, files := fixture(t)
			writeFixture(t, &r, files)
			original := filepath.Join(r.BundleRoot, "timestamp.json")
			external := filepath.Join(t.TempDir(), "external")
			var err error
			switch scenario {
			case "mode":
				err = os.Chmod(original, 0o644)
			case "hardlink":
				err = os.Link(original, external)
			case "symlink":
				err = os.Rename(original, external)
				if err == nil {
					err = os.Symlink(external, original)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Verify(context.Background(), r); !errors.Is(err, ErrInventory) {
				t.Fatalf("native policy: %v", err)
			}
		})
	}
}

func TestUnixNewHardlinkDuringActualReadRefuses(t *testing.T) {
	rootPath := t.TempDir()
	path := filepath.Join(rootPath, "file")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := &observingContext{Context: context.Background(), action: func(call int) {
		if call == 2 {
			if err := os.Link(path, filepath.Join(t.TempDir(), "alias")); err != nil {
				t.Fatal(err)
			}
		}
	}}
	data, _, err := readBundleFile(ctx, root, "file", 3)
	if !errors.Is(err, ErrInventory) || data != nil {
		t.Fatalf("late hardlink: %q %v", data, err)
	}
}
