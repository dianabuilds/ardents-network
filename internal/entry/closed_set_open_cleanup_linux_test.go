//go:build linux

package entry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestOpenClosedSetsJoinsReleaseFailure(t *testing.T) {
	config, _ := closedEntryFixture(t)
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	floor, err := os.ReadFile(filepath.Join(config.Root, "watermark"))
	if err != nil {
		t.Fatal(err)
	}
	config.NetworkID = [32]byte{99}
	result, err := openClosedSetsWithLease(config, func(path string) (rootLease, error) {
		lease, err := acquireRootLease(path)
		if err != nil {
			return lease, err
		}
		if err := lease.file.Close(); err != nil {
			t.Fatal(err)
		}
		return lease, nil
	})
	if result != nil || err == nil || !strings.Contains(err.Error(), "retained authority unavailable") || !errors.Is(err, syscall.EBADF) {
		t.Fatalf("open = %v, %v; want retained authority refusal and physical release failure", result, err)
	}
	got, err := os.ReadFile(filepath.Join(config.Root, "watermark"))
	if err != nil || string(got) != string(floor) {
		t.Fatalf("floor changed: %q, %v", got, err)
	}
}
