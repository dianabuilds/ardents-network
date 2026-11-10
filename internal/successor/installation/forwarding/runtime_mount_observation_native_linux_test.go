//go:build network_signing_native

package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Actual native mount/descriptor facts only; this supplies no process, manager,
// Startup or domain authority. The root driver owns this private mount namespace.
func TestForwardingRuntimeNamedMountNativeMetadata(t *testing.T) {
	marker, err := os.ReadFile("/etc/forwarding-initial-component-marker")
	self, selfErr := os.Stat("/proc/self/ns/mnt")
	manager, managerErr := os.Stat("/proc/1/ns/mnt")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-forwarding-coherent-preparation-v1\n" || selfErr != nil || managerErr != nil || os.SameFile(self, manager) {
		t.Fatal("invalid environment: original private fixture and separate root mount namespace required", err, selfErr, managerErr)
	}
	dir, err := os.MkdirTemp("/native-tmp", "named-mount-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	}()
	writable := filepath.Join(dir, "writable")
	if err := os.Mkdir(writable, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(writable)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(errors.Join(err, file.Close()))
	}
	owner := &inputDirectory{path: writable, file: file, info: info}
	if err := runtimeDirectoryMount(owner, false); err != nil {
		t.Fatal(errors.Join(err, file.Close()))
	}
	if err := os.Mkdir(filepath.Join(writable, "owned-content"), 0700); err != nil {
		t.Fatal(errors.Join(err, file.Close()))
	}
	if err := runtimeDirectoryMount(owner, false); err != nil {
		t.Fatal("directory content changes became immutable metadata", errors.Join(err, file.Close()))
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "immutable")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(path, path, "", syscall.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Unmount(path, 0); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Mount("", path, "", syscall.MS_REMOUNT|syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		t.Fatal(err)
	}
	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	info, err = file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	owner = &inputDirectory{path: path, file: file, info: info}
	if err := runtimeDirectoryMount(owner, true); err != nil {
		t.Fatal("original read-only directory refused", err)
	}
	if err := syscall.Mount(path, path, "", syscall.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	changed := func() (returned error) {
		defer func() { returned = errors.Join(returned, syscall.Unmount(path, 0)) }()
		if err := syscall.Mount("", path, "", syscall.MS_REMOUNT|syscall.MS_BIND, ""); err != nil {
			return err
		}
		named, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(info, named) {
			return errors.New("invalid fixture: new bind changed inode")
		}
		if err := runtimeDirectoryMount(owner, true); !errors.Is(err, ErrBinding) {
			return errors.New("fresh writable mount borrowed retained read-only flags")
		}
		return nil
	}()
	if changed != nil {
		t.Fatal(changed)
	}
	if err := runtimeDirectoryMount(owner, true); err != nil {
		t.Fatal("original fixture mount did not restore", err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	other := *owner
	other.path = alias
	if err := runtimeDirectoryMount(&other, true); !errors.Is(err, syscall.ELOOP) && !errors.Is(err, syscall.ENOTDIR) {
		t.Fatal("fresh named directory followed a symlink", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	if err := runtimeDirectoryMount(owner, true); !errors.Is(err, ErrBinding) {
		t.Fatal("lost retained directory descriptor accepted", err)
	}
}
