//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// This fixture advances filesystem records only; it does not attest actual
// global resource creation, account admission or a stopped system manager.
func archiveStage(t *testing.T) *installationTransaction {
	t.Helper()
	stage := accessStage(t)
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal(err)
	}
	if err := stage.fixedPhase(t.Context(), "0005.json", "publishing-selection"); err != nil {
		t.Fatal(err)
	}
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.createFixedFile(t.Context(), filepath.Join(stage.lease.path, "selection.json"), body, 0640, 65534); err != nil {
		t.Fatal(err)
	}
	if err := stage.fixedPhase(t.Context(), "0006.json", "reloading-manager"); err != nil {
		t.Fatal(err)
	}
	if err := stage.fixedPhase(t.Context(), "0007.json", "installed-stopped"); err != nil {
		t.Fatal(err)
	}
	return stage
}

func TestInstallationNativeIntentArchiveRetainsExactBytesAndRefusesSubstitution(t *testing.T) {
	stage := archiveStage(t)
	original := bytes.Clone(stage.intent.body)
	if err := stage.archiveIntent(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.lease.root.Lstat("transition.json"); !os.IsNotExist(err) {
		t.Fatal("pending intent survived completed archive", err)
	}
	body, err := os.ReadFile(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json"))
	if err != nil || !bytes.Equal(body, original) {
		t.Fatal("archive changed original bytes", err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	if err := stage.archiveIntent(t.Context()); !errors.Is(err, ErrBinding) {
		t.Fatal("completed operation was re-executed", err)
	}
	if err := os.Remove(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := stage.observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("same-byte foreign archive inode accepted", err)
	}
}

func TestInstallationNativeInitialArchiveRefusesStoppedSuccessorIntent(t *testing.T) {
	stage := startBarrierFixture(t)
	original := bytes.Clone(stage.intent.body)
	identity := stage.intent.identity
	if err := stage.archiveIntent(t.Context()); !errors.Is(err, ErrBinding) {
		t.Fatal("initial stopped archive accepted successor before actual start", err)
	}
	if stage.archivedIntent {
		t.Fatal("stopped successor fabricated completed archival")
	}
	current, err := stage.lease.root.Lstat("transition.json")
	if err != nil || !os.SameFile(identity, current) {
		t.Fatal("refusal removed original pending intent", err)
	}
	if err := observeStagedFile(stage.lease.root, "transition.json", stage.intent); err != nil {
		t.Fatal("refusal changed original bytes/access", err)
	}
	if !bytes.Equal(stage.intent.body, original) {
		t.Fatal("refusal changed retained intent")
	}
	if _, err := stage.lease.root.Lstat(filepath.Join("journals", stage.selected.GenerationDigest, "completed-intent.json")); !os.IsNotExist(err) {
		t.Fatal("refusal created a successor archive", err)
	}
}

type archiveCancellation struct {
	context.Context
	stage *installationTransaction
}

func (ctx archiveCancellation) Err() error {
	if _, err := os.Lstat(filepath.Join(ctx.stage.lease.path, "journals", ctx.stage.selected.GenerationDigest, "completed-intent.json")); err == nil {
		return context.Canceled
	}
	return ctx.Context.Err()
}

func TestInstallationNativeIntentArchiveCancellationPreservesPendingOriginal(t *testing.T) {
	stage := archiveStage(t)
	ctx := archiveCancellation{Context: t.Context(), stage: stage}
	if err := stage.archiveIntent(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	if stage.archivedIntent {
		t.Fatal("cancellation fabricated archive completion")
	}
	if err := observeStagedFile(stage.lease.root, "transition.json", stage.intent); err != nil {
		t.Fatal("pending original removed after cancellation", err)
	}
	if _, err := os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json")); err != nil {
		t.Fatal("copy-before-unlink boundary was not reached", err)
	}
	if err := stage.observe(); !errors.Is(err, context.Canceled) {
		t.Fatal("original written cancellation was renewed", err)
	}
}

func TestInstallationNativeIntentArchiveRefusesPrematureOrForeignArchive(t *testing.T) {
	t.Run("premature", func(t *testing.T) {
		stage := accessStage(t)
		if err := stage.archiveIntent(t.Context()); !errors.Is(err, ErrBinding) {
			t.Fatal("incomplete generation archived", err)
		}
		if _, err := os.Lstat(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json")); !os.IsNotExist(err) {
			t.Fatal("archive created before completion", err)
		}
	})
	t.Run("foreign", func(t *testing.T) {
		stage := archiveStage(t)
		if err := os.WriteFile(filepath.Join(stage.lease.path, "journals", stage.selected.GenerationDigest, "completed-intent.json"), stage.intent.body, 0600); err != nil {
			t.Fatal(err)
		}
		if err := stage.archiveIntent(t.Context()); !errors.Is(err, ErrBinding) {
			t.Fatal("foreign archive adopted", err)
		}
		if err := observeStagedFile(stage.lease.root, "transition.json", stage.intent); err != nil {
			t.Fatal("foreign archive retired pending intent", err)
		}
	})
}
