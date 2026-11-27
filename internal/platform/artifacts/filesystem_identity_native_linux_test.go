//go:build linux && ext4_identity_native

package artifacts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This profile requires independently prepared real filesystems. It exercises
// the original descriptors at the artifact input and publication boundaries.
func TestArtifactNativeFilesystemIdentity(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root required")
	}
	for _, kind := range []string{"ext2", "ext3", "ext4", "tmpfs"} {
		t.Run(kind, func(t *testing.T) {
			root := os.Getenv("ARDENTS_TEST_" + strings.ToUpper(kind))
			if root == "" {
				t.Fatal("invalid environment: filesystem root required")
			}
			reported, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", root).Output()
			if err != nil || strings.TrimSpace(string(reported)) != kind {
				t.Fatalf("invalid environment: want %s mount, got %q: %v", kind, reported, err)
			}
			dir, err := os.MkdirTemp(root, "artifact-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			file := filepath.Join(dir, "input")
			if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			inputs := NewInputs(t.Context())
			defer inputs.Close()
			if _, err := inputs.Read(file, 64); err != nil {
				t.Fatal("retain original", err)
			}
			identity := RequireExt4(inputs.Last().Descriptor())
			storage := inputs.CheckProtected()
			output := filepath.Join(dir, "output")
			publication := Publish(t.Context(), output, []Content{{Name: "public", Data: []byte("contents")}}, func() error { return nil }, nil)
			if kind == "ext4" {
				if identity != nil || storage != nil || publication != nil {
					t.Fatalf("ext4 must admit genuine input and output: %v; %v; %v", identity, storage, publication)
				}
				return
			}
			if !errors.Is(identity, ErrNotExt4) || !errors.Is(storage, ErrInput) || !errors.Is(publication, ErrInput) {
				t.Fatalf("%s must refuse at each boundary: %v; %v; %v", kind, identity, storage, publication)
			}
			if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refusal created output: %v", err)
			}
		})
	}
}
