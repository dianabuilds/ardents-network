//go:build linux

package publication

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestOpenPublicationRootJoinsReleaseFailure(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepare", true: "restore"}[restore], func(t *testing.T) {
			path := t.TempDir()
			retained := []byte("unowned evidence")
			name := "foreign"
			primary := "refusing to claim"
			if restore {
				if err := os.WriteFile(filepath.Join(path, rootMarkerName), []byte(rootMarker), 0600); err != nil {
					t.Fatal(err)
				}
				name, primary = floorName, "floor"
				retained = []byte("invalid floor")
			}
			if err := os.WriteFile(filepath.Join(path, name), retained, 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			root, err := openDurableRootWithLease(Config{Root: path}, func(path string) (rootLease, error) {
				calls++
				lease, err := acquireRootLease(path)
				if err != nil {
					return lease, err
				}
				if err := lease.file.Close(); err != nil {
					t.Fatal(err)
				}
				return lease, nil
			})
			if root != nil || err == nil || !strings.Contains(err.Error(), primary) || !errors.Is(err, syscall.EBADF) {
				t.Fatalf("open = %v, %v; want primary refusal and physical release failure", root, err)
			}
			if calls != 1 {
				t.Fatalf("acquisitions = %d", calls)
			}
			got, err := os.ReadFile(filepath.Join(path, name))
			if err != nil || string(got) != string(retained) {
				t.Fatalf("retained evidence changed: %q, %v", got, err)
			}
		})
	}
}
