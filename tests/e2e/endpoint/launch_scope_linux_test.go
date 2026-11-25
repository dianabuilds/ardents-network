//go:build linux && endpoint_launch_native

package endpoint

import (
	"os"
	"strings"
	"testing"
)

func TestLaunchScopeEventReadIsBoundedAndRewinds(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "events")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, tc := range []struct {
		name, body string
		populated  bool
		wantError  bool
	}{
		{"populated", "populated 1\nfrozen 0\n", true, false},
		{"empty", "populated 0\nfrozen 0\n", false, false},
		{"unknown", "frozen 0\n", false, true},
		{"oversized", "populated 1\n" + strings.Repeat("x", 1024), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := file.Truncate(0); err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteAt([]byte(tc.body), 0); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				got, err := LaunchScopePopulated(file)
				if got != tc.populated || (err != nil) != tc.wantError {
					t.Fatalf("populated=%v err=%v", got, err)
				}
			}
		})
	}
}
