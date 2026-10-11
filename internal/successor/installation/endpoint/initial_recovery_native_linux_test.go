//go:build installation_native

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestInstallationNativeRecoveryReadsSelectedArchiveAfterCursorRemoval(t *testing.T) {
	owner := nativeRecoveryLifetime(t, t.Context())
	reader := owner.state.native.reader
	if err := os.Remove(filepath.Join(reader.lease.path, "transition.json")); err != nil {
		t.Fatal(err)
	}
	selected := generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digestHex([]byte("archive-generation")), BindingDigest: digestHex([]byte("archive-binding"))}
	selection, err := canonicalJSON(selected)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(reader.lease.path, "selection.json")
	if err := os.WriteFile(filename, selection, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(filename, 0, int(reader.gid)); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(reader.lease.path, "journals", selected.GenerationDigest)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	wanted, err := canonicalJSON(initialTransitionIntent{Schema: "ardents-endpoint-installation-initial-v1", Candidate: selected})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "completed-intent.json"), wanted, 0600); err != nil {
		t.Fatal(err)
	}
	body, err := readInitialRecoveryIntent(t.Context(), reader)
	if err != nil || !bytes.Equal(body, wanted) {
		t.Fatal("exact selected archive was lost after cursor removal", err)
	}
	if err := os.Symlink(filepath.Join(directory, "completed-intent.json"), filepath.Join(reader.lease.path, "transition.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := readInitialRecoveryIntent(t.Context(), reader); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign pending cursor fell back to selected archive", err)
	}
}

// This lower-level lifetime fixture owns an actual native lease but carries no
// generation authorization or successful manager admission. It tests closure,
// handle copying and cancellation only, not a positive installed operation.
func nativeRecoveryLifetime(t *testing.T, ctx context.Context) *Recovery {
	t.Helper()
	stage := archiveStage(t)
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	reader, err := openInstalledRoot(t.Context(), stage.lease.path)
	if err != nil {
		t.Fatal(err)
	}
	owner := &Recovery{state: &recoveryOperation{ctx: ctx, native: &recoveryNative{reader: reader}}}
	t.Cleanup(func() { _ = owner.Close() })
	return owner
}

func TestInstallationNativeRecoveryCopiesShareOriginalLeaseAndAdmission(t *testing.T) {
	owner := nativeRecoveryLifetime(t, t.Context())
	directory := owner.state.native.reader.lease.path
	copy := *owner
	if other, err := openInstalledRoot(t.Context(), directory); other != nil || !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("retained recovery did not hold original writer", err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Complete(Authorization{}); !errors.Is(err, ErrInput) {
		t.Fatal("handle copy created another completion", err)
	}
	other, err := openInstalledRoot(t.Context(), directory)
	if err != nil {
		t.Fatal("physical close failed to release original lease", err)
	}
	if err := other.lease.close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationNativeRecoveryCannotRenewOriginalCancelledCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner := nativeRecoveryLifetime(t, ctx)
	directory := owner.state.native.reader.lease.path
	cancel()
	if _, err := owner.Complete(Authorization{}); !errors.Is(err, context.Canceled) {
		t.Fatal("another caller renewed the cancelled original operation", err)
	}
	other, err := openInstalledRoot(t.Context(), directory)
	if err != nil {
		t.Fatal("cancelled completion left physical writer live", err)
	}
	if err := other.lease.close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationNativeRecoveryPinsEmptyOwnedPrefixWithoutAdoption(t *testing.T) {
	stage := archiveStage(t)
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	reader, err := openInstalledRoot(t.Context(), stage.lease.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.lease.close(); err != nil {
			t.Error(err)
		}
	})
	filename := filepath.Join(stage.lease.path, "owned-prefix")
	if err := os.WriteFile(filename, nil, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	r := &recoveryNative{reader: reader}
	record := fixedCreationRecord{Device: uint64(native.Dev), Inode: native.Ino}
	wrong := record
	wrong.Inode++
	if err := r.readPrefix(t.Context(), filename, 0644, 0, &wrong); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign birth inode accepted", err)
	}
	if _, recorded := reader.files[filename]; recorded {
		t.Fatal("refused inode was retained as ownership")
	}
	if err := r.readPrefix(t.Context(), filename, 0644, 0, &record); err != nil {
		t.Fatal(err)
	}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("empty owned observation could not be rechecked", err)
	}
	if err := os.Rename(filename, filename+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(reader.observe(t.Context()), ErrBinding) {
		t.Fatal("same-byte replacement acquired original recovery observation")
	}
}

func TestInstallationNativeRecoveryInventoryRequiresCompleteOwnedEntries(t *testing.T) {
	directory := nativeRequestDirectory(t)
	allowed := map[string]bool{"birth.json": true, "failure.json": false}
	if !errors.Is(recoveryInventory(directory, allowed), ErrBinding) {
		t.Fatal("missing required birth accepted")
	}
	if err := os.WriteFile(filepath.Join(directory, "birth.json"), []byte("record\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recoveryInventory(directory, allowed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "foreign.json"), []byte("foreign\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(recoveryInventory(directory, allowed), ErrBinding) {
		t.Fatal("foreign journal entry admitted")
	}
}

// This failure-only fixture owns real files and a writer lease. No fresh proof,
// successful recovery or manager observation is supplied by the fixture.
func TestInstallationNativeRecoveryRetainsFirstFailureAfterItWasArchived(t *testing.T) {
	owner := nativeRecoveryLifetime(t, t.Context())
	native := owner.state.native
	reader := native.reader
	selectedBody, err := reader.read(t.Context(), filepath.Join(reader.lease.path, "selection.json"), 4<<10, 0640, reader.gid)
	if err != nil || decodeCanonical(selectedBody, 4<<10, &native.intent.Candidate) != nil {
		t.Fatal("fixture selection unavailable", err)
	}
	native.journal = filepath.Join(reader.lease.path, "journals", native.intent.Candidate.GenerationDigest)
	first, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: native.intent.Candidate.GenerationDigest, BindingDigest: native.intent.Candidate.BindingDigest, Phase: "initial-recovery-failed", OriginalError: "first interrupted owned repair"})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(native.journal, "recovery-failure-archived.json")
	if err := os.WriteFile(archive, first, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.read(t.Context(), archive, 64<<10, 0600, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Complete(Authorization{}); !errors.Is(err, ErrAuthorization) {
		t.Fatal("later missing authority was not retained in the current outcome", err)
	}
	active, err := os.ReadFile(filepath.Join(native.journal, "recovery-failure.json"))
	if err != nil || !bytes.Equal(active, first) {
		t.Fatal("later refusal displaced the original failure and conflicts with its immutable archive", err)
	}
	retained, err := os.ReadFile(archive)
	if err != nil || !bytes.Equal(retained, first) {
		t.Fatal("original archived failure changed", err)
	}
	// Reopen the actual writer and exercise only the physical archive step.
	// Equal first-error bytes can be synced before the active copy is removed;
	// this lower mechanism supplies no authorization or successful recovery.
	reopened, err := openInstalledRoot(t.Context(), reader.lease.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.lease.close() })
	retry := &recoveryNative{reader: reopened, journal: native.journal}
	for _, filename := range []string{archive, filepath.Join(native.journal, "recovery-failure.json")} {
		if _, err := reopened.read(t.Context(), filename, 64<<10, 0600, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := retry.reader.writePrivate(t.Context(), archive, active); err != nil {
		t.Fatal("retained first error blocks the next exact archive sync", err)
	}
	if err := retry.reader.removeObserved(t.Context(), filepath.Join(native.journal, "recovery-failure.json")); err != nil {
		t.Fatal("joined archive could not retire its active copy", err)
	}
	if _, err := os.Lstat(filepath.Join(native.journal, "recovery-failure.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("active failure survived completed archive step", err)
	}
}
