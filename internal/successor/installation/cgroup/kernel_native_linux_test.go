//go:build installation_native

package cgroup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeStartedScopeRefusesBeforeOpening(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, pid := range []uint32{0, 1} {
		if lifetime, err := RetainStarted(nil, pid, 65534); lifetime != nil || !errors.Is(err, errInput) {
			t.Fatal("missing original caller obtained scope custody", lifetime, err)
		}
		if lifetime, err := RetainStarted(ctx, pid, 65534); lifetime != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled caller obtained pending/start scope custody", lifetime, err)
		}
	}
	if lifetime, err := Retain(t.Context(), 0, 65534); lifetime != nil || !errors.Is(err, errInput) {
		t.Fatal("pending-start exception entered ordinary predecessor pin", lifetime, err)
	}
}

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

func TestInstallationNativeLifetimeRefusesOrdinaryKernelImitation(t *testing.T) {
	name := filepath.Join(nativeDirectory(t), "events")
	if err := os.WriteFile(name, []byte("populated 0\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	lifetime := &Lifetime{pins: []*pin{{events: file, identity: identity}}}
	t.Cleanup(func() { lifetime.Close() })
	if err := lifetime.Observe(); err == nil {
		t.Fatal("ordinary file became live kernel lifetime")
	}
	if joined, err := lifetime.Joined(); err == nil || joined {
		t.Fatal("ordinary file became original join", joined, err)
	}
	if _, err := file.Stat(); err != nil {
		t.Fatal("failed observation released original descriptor", err)
	}
	if err := lifetime.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); err == nil {
		t.Fatal("physical close retained descriptor")
	}
	if joined, err := lifetime.Joined(); err == nil || joined {
		t.Fatal("closed custody became join", joined, err)
	}
}
