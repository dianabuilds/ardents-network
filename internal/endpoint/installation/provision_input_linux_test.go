//go:build linux

package installation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProvisionInputRequiresContextBeforeEffects(t *testing.T) {
	for name, ctx := range map[string]context.Context{"absent": nil} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := loadProvisionInput(ctx, filepath.Join(t.TempDir(), "absent-request")); err == nil {
				t.Fatal("nil-context provision accepted")
			}
		})
	}
}

func TestProvisionRootAncestorControls(t *testing.T) {
	if os.Geteuid() != 0 {
		if err := checkRootAncestors(t.TempDir(), false); err == nil {
			t.Fatal("non-root private root accepted")
		}
		return // Refusal coverage only; root filesystem controls require root.
	}
	base, err := os.MkdirTemp("/root", "ardents-provision-input-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	if err := checkRootAncestors(base, false); err != nil {
		t.Fatal(err)
	}
	if err := requireAbsentManagedPath(filepath.Join(base, "new-parent", "new-file")); err != nil {
		t.Fatal(err)
	}
	if err := requireAbsentManagedPath(base); err == nil {
		t.Fatal("existing managed path adopted")
	}
	if err := checkRootAncestors(filepath.Join(base, "new-floor"), true); err != nil {
		t.Fatal(err)
	}
	if err := checkRootAncestors(filepath.Join(base, "missing-parent", "new-floor"), true); err == nil {
		t.Fatal("missing parent accepted")
	}
	link := filepath.Join(base, "symlink")
	if err := os.Symlink(base, link); err != nil {
		t.Fatal(err)
	}
	if err := checkRootAncestors(link, false); err == nil {
		t.Fatal("symlinked ancestor accepted")
	}
	if err := requireAbsentManagedPath(filepath.Join(link, "new-file")); err == nil {
		t.Fatal("symlinked managed parent accepted")
	}
	if err := os.Chmod(base, 0770); err != nil {
		t.Fatal(err)
	}
	if err := checkRootAncestors(base, false); err == nil {
		t.Fatal("group writable root accepted")
	}
	if err := requireAbsentManagedPath(filepath.Join(base, "new-file")); err == nil {
		t.Fatal("writable managed parent accepted")
	}
}

func TestInstallationManagerOutputIsBounded(t *testing.T) {
	var output commandOutput
	if n, err := output.Write(bytes.Repeat([]byte("x"), 64<<10)); err != nil || n != 64<<10 {
		t.Fatalf("bound write: %d / %v", n, err)
	}
	if n, err := output.Write([]byte("x")); err == nil || n != 0 || output.body.Len() != 64<<10 {
		t.Fatal("manager output exceeded bound")
	}
}
