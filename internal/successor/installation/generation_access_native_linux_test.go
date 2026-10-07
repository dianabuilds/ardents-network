//go:build installation_native

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func accessStage(t *testing.T) *generationStage {
	t.Helper()
	stage := fixedCreationStage(t)
	// This profile exercises metadata and the kernel lease only. It cannot
	// supply successful global fixed files, NSS or manager admission.
	if err := stage.fixedPhase(t.Context(), "0004.json", "fixed-resources-installed"); err != nil {
		t.Fatal(err)
	}
	return stage
}

func TestInstallationNativeGenerationAccessKeepsOriginalLeaseAndInodes(t *testing.T) {
	stage := accessStage(t)
	rootBefore := stage.lease.identity
	parentPath := filepath.Join(stage.lease.path, "generations")
	parentBefore, err := os.Lstat(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal(err)
	}
	for filename, original := range map[string]os.FileInfo{stage.lease.path: rootBefore, parentPath: parentBefore} {
		info, err := os.Lstat(filename)
		if err != nil {
			t.Fatal(err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if !os.SameFile(original, info) || info.Mode() != os.ModeDir|0750 || native.Uid != 0 || native.Gid != 65534 {
			t.Fatal("read access changed original inode or bound owner")
		}
	}
	if err := stage.observe(); err != nil {
		t.Fatal(err)
	}
	if err := stage.promoteAccess(t.Context(), 65534); err != nil {
		t.Fatal("same original access observation refused", err)
	}
	probe, err := os.OpenFile(filepath.Join(stage.lease.path, "writer.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := probe.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("promotion released writer", err)
	}
	if _, err := os.Stat(filepath.Join(stage.lease.path, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("read access selected generation")
	}
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("physical close did not release writer", err)
	}
}

func TestInstallationNativeGenerationAccessRefusesWrongGroupAndCancellation(t *testing.T) {
	stage := accessStage(t)
	if err := stage.promoteAccess(t.Context(), 65533); !errors.Is(err, ErrBinding) {
		t.Fatal("unbound group accepted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := stage.promoteAccess(ctx, 65534); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	for _, filename := range []string{stage.lease.path, filepath.Join(stage.lease.path, "generations")} {
		info, err := os.Lstat(filename)
		if err != nil || !privateJournalDirectory(info) {
			t.Fatal("refused promotion changed metadata", err)
		}
	}
}
