package processresource

import (
	"strings"
	"testing"
)

func TestParseStartClockRequiresOriginalLiveProcess(t *testing.T) {
	raw := "123 (name ) with spaces) S " + strings.Repeat("0 ", 18) + "456 0\n"
	if clock, err := ParseStartClock([]byte(raw), 123); err != nil || clock != 456 {
		t.Fatalf("original clock: %d %v", clock, err)
	}
	for _, bad := range []string{strings.Replace(raw, "123 (", "124 (", 1), strings.Replace(raw, ") S ", ") Z ", 1), strings.Replace(raw, "456", "0", 1), strings.Replace(raw, "456", "0456", 1), "123 (truncated) S 1\n"} {
		if _, err := ParseStartClock([]byte(bad), 123); err == nil {
			t.Fatal("foreign, dead or malformed process admitted")
		}
	}
}
