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
			if restore {
				if err := os.WriteFile(filepath.Join(path, name), retained, 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			root, err := openDurableRootWithLease(Config{Root: path}, func(path string) (rootLease, error) {
				calls++
				lease, err := acquireRootLease(path)
				if err != nil {
					return lease, err
				}
				// Fault the post-acquisition claim check; unsupported roots now refuse
				// before a lease is acquired under the v3 root contract.
				if !restore {
					if err := os.WriteFile(filepath.Join(path, name), retained, 0600); err != nil {
						_ = lease.release()
						t.Fatal(err)
					}
				}
				if err := lease.file.Close(); err != nil {
					t.Fatal(err)
				}
				return lease, nil
			}, syncPublicationDirectory)
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

func TestOpenPublicationRootRetainsSyncAndReleaseCauses(t *testing.T) {
	path := t.TempDir()
	syncFailure := errors.New("injected publication directory sync refusal")
	root, err := openDurableRootWithLease(Config{Root: path}, func(path string) (rootLease, error) {
		lease, err := acquireRootLease(path)
		if err != nil {
			return lease, err
		}
		if err := lease.file.Close(); err != nil {
			t.Fatal(err)
		}
		return lease, nil
	}, func(string) error { return syncFailure })
	if root != nil || !errors.Is(err, syncFailure) || !errors.Is(err, syscall.EBADF) {
		t.Fatalf("open = %v, %v; want sync refusal and physical release cause", root, err)
	}
	marker, err := os.ReadFile(filepath.Join(path, rootMarkerName))
	if err != nil || string(marker) != rootMarker {
		t.Fatalf("failed-open cleanup changed retained marker: %q, %v", marker, err)
	}
}
