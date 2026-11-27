//go:build linux && custody_private_permission_native

package custody

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

// This selected native profile requires four real mounts prepared outside the
// repository. Missing mounts or mount privileges are failures, not skips.
func TestPrivatePermissionMountedFilesystems(t *testing.T) {
	roots := map[string]string{
		"ext2":  os.Getenv("ARDENTS_TEST_EXT2"),
		"ext3":  os.Getenv("ARDENTS_TEST_EXT3"),
		"ext4":  os.Getenv("ARDENTS_TEST_EXT4"),
		"tmpfs": os.Getenv("ARDENTS_TEST_TMPFS"),
	}
	receipt := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: make([]byte, admission.PermissionSize)}
	for _, kind := range []string{"ext2", "ext3", "ext4", "tmpfs"} {
		t.Run(kind, func(t *testing.T) {
			root := roots[kind]
			if root == "" {
				t.Fatal("native filesystem root is required")
			}
			dir, err := os.MkdirTemp(root, "permission-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			path := filepath.Join(dir, "permission")
			err = receipt.PublishAdmissionPermission(path)
			if kind == "ext2" || kind == "ext3" {
				if !errors.Is(err, ErrPrivatePermissionUnavailable) {
					t.Fatalf("unsupported filesystem must refuse before staging: %v", err)
				}
				entries, readErr := os.ReadDir(dir)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("refused delivery left artifacts: %v, %v", entries, readErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("first delivery: %v", err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = receipt.PublishAdmissionPermission(path); err != nil {
				t.Fatalf("exact retry: %v", err)
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("exact retry replaced original: %v", err)
			}
		})
	}

	t.Run("mount replacement uses held object", func(t *testing.T) {
		point, err := os.MkdirTemp(roots["ext4"], "permission-mount-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(point)
		original, err := os.Open(point)
		if err != nil {
			t.Fatal(err)
		}
		defer original.Close()
		if err = syscall.Mount(roots["ext2"], point, "", syscall.MS_BIND, ""); err != nil {
			t.Fatalf("mount replacement prerequisite: %v", err)
		}
		defer func() {
			if err := syscall.Unmount(point, 0); err != nil {
				t.Errorf("unmount replacement: %v", err)
			}
		}()
		if err = checkPrivatePermissionFilesystem(original); err != nil {
			t.Fatalf("held ext4 object changed after replacement: %v", err)
		}
		replacement, err := os.Open(point)
		if err != nil {
			t.Fatal(err)
		}
		defer replacement.Close()
		if err = checkPrivatePermissionFilesystem(replacement); !errors.Is(err, ErrPrivatePermissionUnavailable) {
			t.Fatalf("replacement ext2 object must refuse: %v", err)
		}
	})
}
