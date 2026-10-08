//go:build installation_native

package generation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Actual filesystem custody only: these images supply no journal, fresh Release
// proofs, manager observation or successful Installation recovery.
func prefixFixture(t *testing.T, ctx context.Context) (string, *Prefix, map[string][]byte) {
	t.Helper()
	return prefixImageFixture(t, ctx, nil, 0)
}

func prefixImageFixture(t *testing.T, ctx context.Context, firstBody []byte, firstGID int) (string, *Prefix, map[string][]byte) {
	t.Helper()
	directory, creator := nativeGeneration(t)
	first := "50-ardents-text.rules"
	birth, err := creator.CreateFile(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	if err := creator.Close(); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, first)
	if firstBody != nil {
		if err := os.WriteFile(filename, firstBody, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(filename, 0, firstGID); err != nil {
			t.Fatal(err)
		}
		actual, err := os.Lstat(filename)
		if err != nil || !os.SameFile(birth, actual) {
			t.Fatal("fixture changed birth inode", err)
		}
		birth = actual
	}
	parent := filepath.Dir(directory)
	if err := os.Chown(parent, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0750); err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	p, err := OpenPrefix(ctx, parent, parentInfo, directoryInfo, filepath.Base(directory), 65534, map[string]PrefixFile{first: {Identity: birth, Bytes: firstBody}})
	if err != nil {
		if p != nil {
			_ = p.Close()
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	full := make(map[string][]byte)
	for _, name := range Names() {
		full[name] = []byte("independent complete artifact " + name + "\n")
	}
	return directory, p, full
}

func TestInstallationNativePrefixRepairsOriginalAndExclusivelyBirthsMissingFiles(t *testing.T) {
	t.Parallel()
	directory, p, full := prefixFixture(t, t.Context())
	first := p.files["50-ardents-text.rules"].identity
	born := p.Identity()
	if err := p.Match(full); err != nil {
		t.Fatal(err)
	}
	for _, name := range Names() {
		if name != "50-ardents-text.rules" {
			birth, err := p.CreateFile(name)
			if err != nil || birth.Size() != 0 || birth.Mode() != 0600 {
				t.Fatal("new original leaf not empty/private", err)
			}
		}
		if err := p.Repair(name, full[name]); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Lstat(filepath.Join(directory, "50-ardents-text.rules"))
	if err != nil || !os.SameFile(first, info) {
		t.Fatal("repair replaced original inode", err)
	}
	// Full bytes use resync only; even their modification clock stays unchanged.
	if err := p.Repair("50-ardents-text.rules", full["50-ardents-text.rules"]); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(filepath.Join(directory, "50-ardents-text.rules"))
	if err != nil || !fileObservationMatches(fileObservation{identity: info, body: full["50-ardents-text.rules"], mode: 0640, gid: 65534}, after) {
		t.Fatal("complete artifact was rewritten", err)
	}
	if err := p.Seal(); err != nil {
		t.Fatal(err)
	}
	sealed := p.Identity()
	if sealed.Device != born.Device || sealed.Inode != born.Inode || sealed.Mode != os.ModeDir|0750 || sealed.GID != 65534 {
		t.Fatal("seal substituted original directory")
	}
	retained := p.files["50-ardents-text.rules"].file
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := retained.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original leaf descriptor survived Close", err)
	}
	parent, err := os.Lstat(filepath.Dir(directory))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := OpenSnapshot(t.Context(), filepath.Dir(directory), parent, dir, filepath.Base(directory), 65534)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	for name, body := range full {
		if !bytes.Equal(snapshot.Bytes(name), body) {
			t.Fatal("independent sealed reopen differs", name)
		}
	}
}

func TestInstallationNativePrefixRetainsOriginalCancellationAndForeignInodeFailure(t *testing.T) {
	t.Parallel()
	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		directory, p, full := prefixFixture(t, ctx)
		retained := p.files["50-ardents-text.rules"].file
		cancel()
		if err := p.Repair("50-ardents-text.rules", full["50-ardents-text.rules"]); !errors.Is(err, context.Canceled) {
			t.Fatal("caller renewed", err)
		}
		info, err := os.Lstat(filepath.Join(directory, "50-ardents-text.rules"))
		if err != nil || info.Size() != 0 {
			t.Fatal("cancelled caller wrote bytes", err)
		}
		if _, err := retained.Stat(); err != nil {
			t.Fatal("failed operation prematurely closed custody", err)
		}
		if err := p.Close(); !errors.Is(err, context.Canceled) {
			t.Fatal("Close lost first caller failure", err)
		}
		if _, err := retained.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("cancelled original descriptor not closed", err)
		}
	})
	t.Run("same-byte-foreign-inode", func(t *testing.T) {
		directory, p, full := prefixFixture(t, t.Context())
		filename := filepath.Join(directory, "50-ardents-text.rules")
		retained := p.files["50-ardents-text.rules"].file
		originalPath := filepath.Join(filepath.Dir(directory), "original-leaf")
		if err := os.Rename(filename, originalPath); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := p.Observe(); !errors.Is(err, ErrBinding) {
			t.Fatal("foreign inode accepted", err)
		}
		if err := os.Remove(filename); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(originalPath, filename); err != nil {
			t.Fatal(err)
		}
		if err := p.Repair("50-ardents-text.rules", full["50-ardents-text.rules"]); !errors.Is(err, ErrBinding) {
			t.Fatal("restored path cleared first failure", err)
		}
		if _, err := retained.Stat(); err != nil {
			t.Fatal("original descriptor released before Close", err)
		}
		if err := p.Close(); !errors.Is(err, ErrBinding) {
			t.Fatal("foreign failure lost", err)
		}
	})
}

func TestInstallationNativePrefixRefusesDetachedAndIncompleteImages(t *testing.T) {
	t.Parallel()
	if err := (&Prefix{}).Observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("detached owner accepted", err)
	}
	if err := (&Prefix{}).Close(); err != nil {
		t.Fatal("detached close failed", err)
	}
	_, p, full := prefixFixture(t, t.Context())
	delete(full, "binding.json")
	if err := p.Match(full); !errors.Is(err, ErrBinding) {
		t.Fatal("missing full preimage accepted", err)
	}
	// A later complete map cannot clear this first refusal.
	full["binding.json"] = []byte(strings.Repeat("a", 20))
	if err := p.Match(full); !errors.Is(err, ErrBinding) {
		t.Fatal("first refusal renewed")
	}
}

func TestInstallationNativePrefixRepairsTornBytesAndInterruptedAccessPromotion(t *testing.T) {
	t.Parallel()
	first := "50-ardents-text.rules"
	wanted := []byte("independent complete artifact " + first + "\n")
	for _, test := range []struct {
		name  string
		image []byte
		gid   int
	}{
		{"torn-private-bytes", []byte("independent complete art"), 0},
		{"chown-before-chmod", wanted, 65534},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory, p, full := prefixImageFixture(t, t.Context(), test.image, test.gid)
			original := p.files[first].identity
			if err := p.Match(full); err != nil {
				t.Fatal(err)
			}
			if err := p.Repair(first, full[first]); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(filepath.Join(directory, first))
			body, readErr := os.ReadFile(filepath.Join(directory, first))
			if err != nil || readErr != nil || !os.SameFile(original, info) || info.Mode() != 0640 || !bytes.Equal(body, wanted) {
				t.Fatal("original prefix repair differs", err, readErr)
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInstallationNativePrefixRefusesForeignPreimageBeforeAnyRepair(t *testing.T) {
	t.Parallel()
	directory, p, full := prefixImageFixture(t, t.Context(), []byte("foreign prefix"), 0)
	if err := p.Match(full); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign bytes matched complete preimage", err)
	}
	if err := p.Repair("50-ardents-text.rules", full["50-ardents-text.rules"]); !errors.Is(err, ErrBinding) {
		t.Fatal("first foreign refusal renewed", err)
	}
	actual, err := os.ReadFile(filepath.Join(directory, "50-ardents-text.rules"))
	if err != nil || !bytes.Equal(actual, []byte("foreign prefix")) {
		t.Fatal("refused prefix was mutated", err)
	}
}
