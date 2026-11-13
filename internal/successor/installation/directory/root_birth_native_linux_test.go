//go:build installation_native

package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeRootBirthDeliveryAndSeal(t *testing.T) {
	parent := nativeDirectory(t)
	before, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "generation")
	owned, err := CreateRoot(t.Context(), path, before)
	if owned != nil {
		defer owned.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	birth, err := owned.Identity()
	if err != nil || !rootBirthAccess(birth, 0700, 0, 0) {
		t.Fatal("exclusive root birth missing", err)
	}
	delivered, err := owned.Deliver(0755, 0, 0)
	if err != nil || !os.SameFile(birth, delivered) || !rootBirthAccess(delivered, 0755, 0, 0) {
		t.Fatal("delivery changed original inode", err)
	}
	if _, err := owned.Deliver(0755, 0, 0); err == nil {
		t.Fatal("delivery was repeated")
	}
	// A genuine child birth changes ctime and link counts. Those belong to
	// Installation's inventory; this mechanism still retains the parent inode.
	if err := os.WriteFile(filepath.Join(path, "artifact"), []byte("immutable bytes\n"), 0444); err != nil {
		t.Fatal(err)
	}
	sealed, err := owned.SealRoot()
	if err != nil || !os.SameFile(birth, sealed) || !rootBirthAccess(sealed, 0555, 0, 0) {
		t.Fatal("same-inode seal failed", err)
	}
	if _, err := owned.SealRoot(); err == nil {
		t.Fatal("seal was repeated")
	}
	fd := owned.file
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := fd.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("directory survived joined Close", err)
	}
	if err := owned.Observe(); err == nil {
		t.Fatal("closed owner observed again")
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("Close removed retained data", err)
	}
	foreign, err := CreateRoot(t.Context(), path, before)
	if err == nil {
		t.Fatal("existing directory adopted")
	}
	if foreign != nil && foreign.Close() == nil {
		t.Fatal("original exclusive birth failure erased")
	}
}

func TestInstallationNativeRootBirthCancellationAndSubstitution(t *testing.T) {
	for _, kind := range []string{"cancel", "substitute", "nonempty"} {
		t.Run(kind, func(t *testing.T) {
			parent := nativeDirectory(t)
			before, err := os.Lstat(parent)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			path := filepath.Join(parent, "root")
			owned, err := CreateRoot(ctx, path, before)
			if owned != nil {
				defer owned.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			birth, err := owned.Identity()
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "cancel":
				cancel()
			case "substitute":
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "nonempty":
				if err := os.WriteFile(filepath.Join(path, "foreign"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := owned.Deliver(0755, 0, 0); err == nil {
				t.Fatal("invalid original admitted delivery")
			}
			result := owned.Close()
			if result == nil || owned.Close() != result {
				t.Fatal("original failure not retained", result)
			}
			info, err := os.Lstat(path)
			if err != nil || info.Mode().Perm() != 0700 {
				t.Fatal("failed delivery changed access", err)
			}
			if kind != "substitute" && !os.SameFile(birth, info) {
				t.Fatal("refused original was replaced")
			}
		})
	}
}
