//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestForwardingBirthsNativeOriginalJournalExcludesFreshRetry(t *testing.T) {
	f := prepareInputFixture(t)
	path := filepath.Join(filepath.Dir(f.path), "initial-journal")
	j, err := openForwardingBirths(t.Context(), path, map[string]bool{path: true}, "forwarding-initial-v1")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err == nil {
		t.Fatal("original writer lease did not exclude an independent reader")
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined writer did not return its actual lease", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		retry, err := openForwardingBirths(t.Context(), path, map[string]bool{path: true}, "forwarding-initial-v1")
		if err == nil {
			t.Fatal("retained original journal reopened as a fresh transaction")
		}
		if err := retry.close(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("fresh retry changed original prefix", err)
	}
}

func TestForwardingBirthsNativeWritesAndClosesOriginalResources(t *testing.T) {
	f := prepareInputFixture(t)
	parent := filepath.Dir(f.path)
	journal, dir, leaf := filepath.Join(parent, "journal"), filepath.Join(parent, "generation"), filepath.Join(parent, "generation", "artifact")
	j, err := openForwardingBirths(t.Context(), journal, map[string]bool{journal: true, dir: true, leaf: true}, "forwarding-initial-v1")
	if err != nil {
		t.Fatal(err)
	}
	if err = j.directory(dir, 0755, 0, 0, j.check); err != nil {
		t.Fatal(err)
	}
	if err = j.write(leaf, []byte("fixed exact artifact\n"), 0444, 0, 0, j.check); err != nil {
		t.Fatal(err)
	}
	if err = j.check(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(leaf)
	if err != nil || !nodeAccess(info, 0444, 0, 0, false) {
		t.Fatal(err)
	}
	fd := j.files[0].file
	if err = j.close(); err != nil {
		t.Fatal(err)
	}
	if _, err = fd.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original file remained open after joined close", err)
	}
	if err = j.check(); err == nil {
		t.Fatal("closed creator admitted another effect")
	}
}

func TestForwardingBirthsNativeUncertainSyncRetainsOriginalFailure(t *testing.T) {
	f := prepareInputFixture(t)
	parent := filepath.Dir(f.path)
	journal, leaf := filepath.Join(parent, "journal"), filepath.Join(parent, "artifact")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	j, err := openForwardingBirths(ctx, journal, map[string]bool{journal: true, leaf: true}, "forwarding-initial-v1")
	if err != nil {
		t.Fatal(err)
	}
	physical := errors.New("actual sync completed but publication result unavailable")
	j.afterSync = func(file *os.File) error {
		if file.Name() == leaf {
			cancel()
			return physical
		}
		return nil
	}
	if err = j.write(leaf, []byte("retained physical prefix\n"), 0444, 0, 0, j.check); !errors.Is(err, physical) {
		t.Fatal("first physical result lost", err)
	}
	result := j.close()
	if !errors.Is(result, physical) || !errors.Is(result, context.Canceled) {
		t.Fatal("original sync/caller failures not retained", result)
	}
	if _, err := os.Lstat(leaf); err != nil {
		t.Fatal("uncertain original effect was silently removed", err)
	}
	if j.close() != result {
		t.Fatal("repeated close changed original joined result")
	}
}
