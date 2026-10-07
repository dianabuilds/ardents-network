//go:build installation_native

package installation

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
	reader, err := openInstalledInspection(t.Context(), stage.lease.path)
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
	if other, err := openInstalledInspection(t.Context(), directory); other != nil || !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("retained recovery did not hold original writer", err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Complete(Authorization{}); !errors.Is(err, ErrInput) {
		t.Fatal("handle copy created another completion", err)
	}
	other, err := openInstalledInspection(t.Context(), directory)
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
	other, err := openInstalledInspection(t.Context(), directory)
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
	reader, err := openInstalledInspection(t.Context(), stage.lease.path)
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
