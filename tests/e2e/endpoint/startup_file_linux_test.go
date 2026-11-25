//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishPermissionResponsePreservesOriginalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "participant", "response")
	uid, gid := uint32(os.Geteuid()), uint32(os.Getegid())
	if err := PublishPermissionResponse(t, path, []byte("original"), uid, gid); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if original.Mode().Perm() != 0600 {
		t.Fatalf("permission response mode = %v", original.Mode())
	}
	if err := PublishPermissionResponse(t, path, []byte("replacement"), uid, gid); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second handover = %v, want already exists", err)
	}
	retained, err := os.Stat(path)
	if err != nil || !os.SameFile(original, retained) {
		t.Fatalf("original response inode not retained: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "original" {
		t.Fatalf("original response not retained: %q, %v", raw, err)
	}
	if _, err := os.Lstat(path + ".pending"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected pending response after refusal: %v", err)
	}
}
