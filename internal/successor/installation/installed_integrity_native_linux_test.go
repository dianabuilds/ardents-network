//go:build installation_native

package installation

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

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
	reader := &installedInspection{mutableDirectories: map[string]os.FileInfo{}}
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
