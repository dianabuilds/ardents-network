//go:build installation_native

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestInstallationNativeLeaseHoldsUntilPhysicalClose(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "installation")
	lease, err := createInitialLease(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.OpenFile(filepath.Join(directory, "writer.lock"), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := probe.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("kernel lease not retained: %v", err)
	}
	if other, err := createInitialLease(t.Context(), directory); err == nil || other != nil {
		t.Fatal("existing root adopted")
	}
	if err := lease.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("closed owner retained kernel lease: %v", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := lease.close(); err != nil {
		t.Fatal("repeat close changed terminal result")
	}
}

func TestInstallationNativeLeaseRefusesBirthAndIdentityChanges(t *testing.T) {
	for _, name := range []string{"writable-parent", "existing-symlink", "writer-hardlink", "writer-substitution", "root-substitution", "root-mode"} {
		t.Run(name, func(t *testing.T) {
			parent := nativeRequestDirectory(t)
			directory := filepath.Join(parent, "installation")
			if name == "writable-parent" {
				if err := os.Chmod(parent, 0770); err != nil {
					t.Fatal(err)
				}
				if lease, err := createInitialLease(t.Context(), directory); err == nil || lease != nil {
					t.Fatal("writable parent admitted")
				}
				if _, err := os.Lstat(directory); !os.IsNotExist(err) {
					t.Fatal("preflight refusal created root")
				}
				return
			}
			if name == "existing-symlink" {
				if err := os.Symlink(parent, directory); err != nil {
					t.Fatal(err)
				}
				if lease, err := createInitialLease(t.Context(), directory); err == nil || lease != nil {
					t.Fatal("symlink adopted")
				}
				return
			}
			lease, err := createInitialLease(t.Context(), directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := lease.close(); err != nil {
					t.Error(err)
				}
			}()
			writer := filepath.Join(directory, "writer.lock")
			var mutationErr error
			switch name {
			case "writer-hardlink":
				mutationErr = os.Link(writer, filepath.Join(parent, "alias"))
			case "writer-substitution":
				mutationErr = os.Rename(writer, writer+".original")
				if mutationErr == nil {
					mutationErr = os.WriteFile(writer, nil, 0600)
				}
			case "root-substitution":
				mutationErr = os.Rename(directory, directory+".original")
				if mutationErr == nil {
					mutationErr = os.Mkdir(directory, 0700)
				}
			case "root-mode":
				mutationErr = os.Chmod(directory, 0750)
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			if err := lease.observe(); err == nil {
				t.Fatal("changed root/lease identity admitted")
			}
		})
	}
}

func TestInstallationNativeRequestCustodyReobservation(t *testing.T) {
	for _, name := range []string{"same", "changed-bytes", "replaced-inode", "lost-file"} {
		t.Run(name, func(t *testing.T) {
			filename := writeNativeRequest(t, nativeRequestDirectory(t))
			request, err := ReadOwnedRequest(t.Context(), filename, true)
			if err != nil {
				t.Fatal(err)
			}
			var mutationErr error
			switch name {
			case "changed-bytes":
				mutationErr = os.WriteFile(filename, []byte("{}\n"), 0600)
			case "replaced-inode":
				mutationErr = os.Rename(filename, filename+".original")
				if mutationErr == nil {
					mutationErr = os.WriteFile(filename, requestFixture(), 0600)
				}
			case "lost-file":
				mutationErr = os.Remove(filename)
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			err = observeRequestCustody(t.Context(), request)
			if name == "same" && err != nil || name != "same" && err == nil {
				t.Fatalf("custody reobservation = %v", err)
			}
		})
	}
}
