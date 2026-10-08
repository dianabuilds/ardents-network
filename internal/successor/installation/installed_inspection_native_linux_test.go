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

// This exercises creation/read custody on actual native files only. The
// staging fixture grants no Release, account, manager or startup authority.
func TestInstallationNativeCandidateReadCustodyBindsOriginalStage(t *testing.T) {
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stage.close() })
	binding, observed, err := reader.retainStagedGeneration(t.Context(), stage)
	if err != nil || !bytes.Equal(binding, files["binding.json"]) || len(observed) != len(files)-1 {
		t.Fatal("original candidate was not retained", err)
	}
	program := filepath.Join(reader.lease.path, "generations", selected.GenerationDigest, "ardents-linux-amd64")
	original := reader.files[program]
	if original.identity == nil || !bytes.Equal(original.body, files["ardents-linux-amd64"]) {
		t.Fatal("candidate process executable has no original read observation")
	}
	observed["ardents-linux-amd64"][0] ^= 1
	binding[0] ^= 1
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("detached returned bytes changed original read custody", err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal("detached returned bytes changed creation custody", err)
	}
	if err := os.Rename(program, filepath.Join(reader.lease.path, "original-program")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, original.body, original.mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(program, 0, int(original.gid)); err != nil {
		t.Fatal(err)
	}
	if binding, observed, err := reader.retainStagedGeneration(t.Context(), stage); !errors.Is(err, ErrBinding) || binding != nil || observed != nil {
		t.Fatal("same-byte candidate executable renewed read custody", err)
	}
	if !os.SameFile(original.identity, reader.files[program].identity) {
		t.Fatal("refusal replaced original process file observation")
	}
}

func TestInstallationNativeCandidateReadCustodyRefusesCancelledAndForeignLease(t *testing.T) {
	reader, request, previous, files, selected := successorStagingFixture(t)
	stage, err := stageSuccessorGeneration(t.Context(), reader, request, previous, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stage.close() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if binding, observed, err := reader.retainStagedGeneration(ctx, stage); !errors.Is(err, context.Canceled) || binding != nil || observed != nil {
		t.Fatal("cancelled caller retained candidate", err)
	}
	// Copying the original handles cannot create a second owner. This deliberate
	// borrowed-custody refusal fixture grants no native transition authority.
	borrowedLease := *reader.lease
	other := &installedInspection{lease: &borrowedLease, installedFiles: reader.installedFiles}
	if binding, observed, err := other.retainStagedGeneration(t.Context(), stage); !errors.Is(err, ErrBinding) || binding != nil || observed != nil {
		t.Fatal("foreign lease borrowed original candidate custody", err)
	}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal("refusal changed predecessor read custody", err)
	}
	if err := stage.observe(); err != nil {
		t.Fatal("refusal changed candidate creation custody", err)
	}
}

func TestInstallationNativeInspectionCannotRenewGenerationDirectory(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "generations")
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(directory, 0, 65534); err != nil {
		t.Fatal(err)
	}
	reader := &installedFiles{gid: 65534, directories: make(map[string]os.FileInfo)}
	if err := reader.pinGenerationDirectory(directory); err != nil {
		t.Fatal(err)
	}
	original := reader.directories[directory]
	if err := os.Rename(directory, directory+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(directory, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := reader.pinGenerationDirectory(directory); !errors.Is(err, ErrBinding) {
		t.Fatal("new inode renewed original directory custody", err)
	}
	if !os.SameFile(original, reader.directories[directory]) {
		t.Fatal("refusal replaced original directory observation")
	}
}

func TestInstallationNativeInspectionRetainsOriginalKernelLease(t *testing.T) {
	stage := archiveStage(t)
	directory := stage.lease.path
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	reader, err := openInstalledInspection(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.lease.close(); err != nil {
			t.Error(err)
		}
	})
	if other, err := openInstalledInspection(t.Context(), directory); !errors.Is(err, syscall.EWOULDBLOCK) || other != nil {
		t.Fatal("inspection did not serialize with original writer", err)
	}
	filename := filepath.Join(directory, "selection.json")
	body, err := reader.read(t.Context(), filename, 4<<10, 0640, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("inspection did not read actual selection")
	}
	if err := reader.observe(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := reader.lease.close(); err != nil {
		t.Fatal(err)
	}
	other, err := openInstalledInspection(t.Context(), directory)
	if err != nil {
		t.Fatal("physical close did not return writer", err)
	}
	if err := other.lease.close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationNativeInspectionRefusesSameByteForeignInode(t *testing.T) {
	stage := archiveStage(t)
	directory := stage.lease.path
	if err := stage.close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.lease.close(); err != nil {
		t.Fatal(err)
	}
	reader, err := openInstalledInspection(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.lease.close(); err != nil {
			t.Error(err)
		}
	})
	filename := filepath.Join(directory, "selection.json")
	body, err := reader.read(t.Context(), filename, 4<<10, 0640, 65534)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, bytes.Clone(body), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(filename, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := reader.observe(t.Context()); !errors.Is(err, ErrBinding) {
		t.Fatal("same-byte foreign selection accepted", err)
	}
}

func TestInstallationNativeInspectionRetainsMutableAncestorIdentity(t *testing.T) {
	directory := nativeRequestDirectory(t)
	parent := filepath.Join(directory, "mutable-parent")
	child := filepath.Join(parent, "root")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	originalParent, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	originalChild, err := os.Lstat(child)
	if err != nil {
		t.Fatal(err)
	}
	reader := &installedFiles{mutableDirectories: map[string]os.FileInfo{}}
	if err := reader.retainMutableDirectory(parent, originalParent); err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(directory, "retained-parent")
	if err := os.Rename(parent, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(retained, "root"), child); err != nil {
		t.Fatal(err)
	}
	currentChild, err := os.Lstat(child)
	if err != nil || !os.SameFile(originalChild, currentChild) {
		t.Fatal("control did not preserve mutable leaf inode", err)
	}
	currentParent, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.retainMutableDirectory(parent, currentParent); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign ancestor accepted with unchanged leaf", err)
	}
}
