//go:build linux

package installation

import "testing"

func TestInstallationPlatformRequiresMatchingLTSManager(t *testing.T) {
	for _, test := range []struct {
		release string
		manager uint16
		accept  bool
	}{
		{"22.04", 249, true}, {"24.04", 255, true},
		{"22.04", 255, false}, {"24.04", 249, false},
		{"26.04", 259, false}, {"24.04", 259, false},
		{"22.04", 0, false}, {"24.04", 0, false}, {"", 249, false},
	} {
		if got := installationPlatformPair(test.release, test.manager); got != test.accept {
			t.Errorf("release %q manager %d: accepted=%t, want %t", test.release, test.manager, got, test.accept)
		}
	}
}
