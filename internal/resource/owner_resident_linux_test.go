//go:build linux

package resource

import "testing"

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
