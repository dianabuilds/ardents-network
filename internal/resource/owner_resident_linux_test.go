//go:build linux

package resource

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOwnerResidentBytesRetriesBoundedDisappearingProcesses(t *testing.T) {
	group := t.TempDir()
	inventories, statmReads := 0, 0
	got, err := ownerResidentBytesWithReader([]string{group}, func(path string, _ int) (string, error) {
		switch path {
		case filepath.Join(group, "cgroup.procs"):
			inventories++
			switch inventories {
			case 1:
				return "42\n", nil
			case 2:
				return "43\n", nil
			default:
				return "44\n", nil
			}
		case filepath.Join("/proc", "42", "statm"):
			statmReads++
			return "", os.ErrNotExist
		case filepath.Join("/proc", "43", "statm"):
			statmReads++
			return "", syscall.ESRCH
		case filepath.Join("/proc", "44", "statm"):
			statmReads++
			return "10 3 0 0 0 0 0", nil
		default:
			return "", errors.New("unexpected path")
		}
	})
	if err != nil || got != 3*uint64(os.Getpagesize()) || inventories != 3 || statmReads != 3 {
		t.Fatalf("resident retry = %d, %v after %d inventories and %d statm reads", got, err, inventories, statmReads)
	}
}

func TestOwnerResidentBytesRejectsRepeatedOrNonProcessInventoryFailure(t *testing.T) {
	group := t.TempDir()
	repeated := 0
	if _, err := ownerResidentBytesWithReader([]string{group}, func(path string, _ int) (string, error) {
		if path == filepath.Join(group, "cgroup.procs") {
			return "42\n", nil
		}
		repeated++
		return "", os.ErrNotExist
	}); err == nil || repeated != ownerResidentSampleAttempts {
		t.Fatalf("repeated disappearance = %v after %d reads", err, repeated)
	}
	cgroupReads := 0
	if _, err := ownerResidentBytesWithReader([]string{group}, func(path string, _ int) (string, error) {
		cgroupReads++
		return "", os.ErrNotExist
	}); err == nil || cgroupReads != 1 {
		t.Fatalf("inventory disappearance = %v after %d reads", err, cgroupReads)
	}
	statmReads := 0
	if _, err := ownerResidentBytesWithReader([]string{group}, func(path string, _ int) (string, error) {
		if path == filepath.Join(group, "cgroup.procs") {
			return "42\n", nil
		}
		statmReads++
		return "", os.ErrPermission
	}); err == nil || statmReads != 1 {
		t.Fatalf("non-ENOENT statm failure = %v after %d reads", err, statmReads)
	}
}

func TestOwnerResidentPagesRejectMalformedAndOverflowingCounters(t *testing.T) {
	if got, err := residentBytes("1000 17 0 0 0 0 0", 4096); err != nil || got != 69632 {
		t.Fatalf("resident pages: %d %v", got, err)
	}
	for _, sample := range []string{"", "1 2", "1 -2 0 0 0 0 0", "1 18446744073709551615 0 0 0 0 0", "1 NaN 0 0 0 0 0"} {
		if _, err := residentBytes(sample, 4096); err == nil {
			t.Errorf("accepted %q", sample)
		}
	}
	if _, err := residentBytes("1 2 0 0 0 0 0", 0); err == nil {
		t.Fatal("accepted zero page size")
	}
}
