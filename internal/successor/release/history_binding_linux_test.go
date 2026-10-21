package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryPathMatchesOriginalVerifierNotEquivalentFloors(t *testing.T) {
	h := freshSignedHistory(t)
	paths := []string{filepath.Join(t.TempDir(), "original"), filepath.Join(t.TempDir(), "foreign")}
	owners := make([]*Verifier, len(paths))
	for i, path := range paths {
		v, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		owners[i] = v
		t.Cleanup(func() {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		})
		if d := v.Evaluate(t.Context(), h.input); d.Outcome != OutcomeReleaseAccepted {
			t.Fatal(d.Outcome, d.Err())
		}
		if err := v.CheckHistoryPath(t.Context(), path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{paths[1], "relative", ""} {
		if err := owners[0].CheckHistoryPath(t.Context(), path); !errors.Is(err, ErrTrustUnavailable) {
			t.Fatal("foreign or noncanonical path accepted", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := owners[0].CheckHistoryPath(ctx, paths[0]); !errors.Is(err, context.Canceled) {
		t.Fatal("original caller cancellation lost", err)
	}
	if err := owners[0].Close(); err != nil {
		t.Fatal(err)
	}
	if err := owners[0].CheckHistoryPath(t.Context(), paths[0]); !errors.Is(err, ErrClosed) {
		t.Fatal("closed verifier accepted", err)
	}
}

func TestHistoryPathRejectsEquivalentReplacementBeforeConsumerPins(t *testing.T) {
	h := freshSignedHistory(t)
	parent := t.TempDir()
	path, other := filepath.Join(parent, "original"), filepath.Join(parent, "equivalent")
	v, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	w, err := Open(other)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []*Verifier{v, w} {
		if d := owner.Evaluate(t.Context(), h.input); d.Outcome != OutcomeReleaseAccepted {
			t.Fatal(d.Outcome, d.Err())
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	// The copied history is independently leasable while v still owns the old
	// lease. Equal floors and the canonical pathname cannot establish identity.
	replacement, err := OpenRetained(path)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if err := replacement.CheckHistoryPath(t.Context(), path); err != nil {
		t.Fatal("genuine replacement verifier refused its own directory", err)
	}
	if err := v.CheckHistoryPath(t.Context(), path); !errors.Is(err, ErrTrustUnavailable) {
		t.Fatal("equivalent replacement accepted before consumer acquired pins", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, other); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".original", path); err != nil {
		t.Fatal(err)
	}
	if err := v.CheckHistoryPath(t.Context(), path); !errors.Is(err, ErrTrustUnavailable) {
		t.Fatal("restoring pathname erased original identity failure", err)
	}
	if err := v.Close(); !errors.Is(err, ErrTrustUnavailable) {
		t.Fatal("original identity failure lost during joined close", err)
	}
}

func TestHistoryPathRejectsReplacedOriginalLease(t *testing.T) {
	h := freshSignedHistory(t)
	parent := t.TempDir()
	path := filepath.Join(parent, "history")
	v, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if d := v.Evaluate(t.Context(), h.input); d.Outcome != OutcomeReleaseAccepted {
		t.Fatal(d.Outcome, d.Err())
	}
	lock := filepath.Join(path, floorStoreLockName)
	if err := os.Rename(lock, filepath.Join(parent, "original-lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(floorStoreLeaseContents), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := OpenRetained(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.CheckHistoryPath(t.Context(), path); err != nil {
		t.Fatal(err)
	}
	if err := v.CheckHistoryPath(t.Context(), path); !errors.Is(err, ErrTrustUnavailable) {
		t.Fatal("replacement lock inode accepted as original lease", err)
	}
}
