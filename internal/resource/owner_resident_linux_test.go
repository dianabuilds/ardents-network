//go:build linux

package resource

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The private directory supplies inventory bytes only; it is not a cgroup or
// accepting native scope. Actual Linux procfs supplies this process's pages and
// the missing-process failure. A partial inventory must never become a sample.
func TestOwnerResidentMissingProcessRefusesEntireSample(t *testing.T) {
	root := t.TempDir()
	inventory := filepath.Join(root, "cgroup.procs")
	if err := os.WriteFile(inventory, []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := ownerResidentBytes([]string{root}); err != nil || got == 0 {
		t.Fatal("original process did not supply resident pages", got, err)
	}
	// Linux pid_t is signed; this uint32 value cannot name a kernel process.
	if err := os.WriteFile(inventory, []byte(strconv.Itoa(os.Getpid())+"\n4294967295\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := ownerResidentBytes([]string{root}); got != 0 || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing process produced a partial accepting sample", got, err)
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
