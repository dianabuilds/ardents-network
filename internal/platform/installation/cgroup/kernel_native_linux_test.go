//go:build installation_native

package cgroup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Valid-looking bytes on an ordinary filesystem are not kernel completion.
// This failure-only control supplies no successful cgroup or stop authority.
func TestInstallationNativeCgroupPinRefusesFilesystemImitation(t *testing.T) {
	directory := nativeDirectory(t)
	filename := filepath.Join(directory, "events-imitation")
	if err := os.WriteFile(filename, []byte("populated 0\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	pin := &pin{events: file}
	t.Cleanup(func() {
		if err := pin.close(); err != nil {
			t.Error(err)
		}
	})
	identity, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pin.identity = identity
	if removed, populated, err := pin.read(); err == nil || removed || populated {
		t.Fatalf("ordinary filesystem became kernel completion: removed=%v populated=%v error=%v", removed, populated, err)
	}
	if err := pin.close(); err != nil {
		t.Fatal(err)
	}
	if removed, populated, err := pin.read(); err == nil || removed || populated {
		t.Fatalf("closed descriptor became kernel completion: removed=%v populated=%v error=%v", removed, populated, err)
	}
}

func TestInstallationNativeEmptyInventoryRefusesOrdinaryFilesystem(t *testing.T) {
	// An actual root-owned empty directory is a failure control, not a kernel
	// cgroup namespace, manager invocation or successfully joined lifetime.
	root, err := os.OpenRoot(nativeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	if names, err := readScopeNames(t.Context(), root); !errors.Is(err, errBinding) || names != nil {
		t.Fatal("ordinary empty directory became kernel inventory", names, err)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatal("inventory reader closed caller's directory custody", err)
	}
}

func nativeDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" {
		t.Fatal("invalid environment: installation_native requires root and a trusted temporary parent")
	}
	if err := observeAncestors(parent); err != nil {
		t.Fatal("invalid temporary parent", err)
	}
	directory, err := os.MkdirTemp(parent, "installation-cgroup-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}
