//go:build darwin

package hosting

import (
	"errors"
	"testing"
)

// macOS measurement is staged unavailable (#865): the routing-socket if_data64
// byte counters wrap at 2^32 and cannot preserve per-boot per-interface monotonic
// continuity, so the adapter refuses with the typed platform error rather than
// accept a weaker counter source. A future port must read the full 64-bit IFMIB
// interface-data totals and re-prove the continuity facts.
func TestDarwinMeasureRefusesUnsupportedPlatform(t *testing.T) {
	if err := hostingPlatform(); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("hostingPlatform = %v, want ErrUnsupportedPlatform", err)
	}
	if Supported() {
		t.Fatal("Supported reported macOS measurement available")
	}
	if _, err := measureHosting([]string{"lo0"}); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("measureHosting = %v, want ErrUnsupportedPlatform", err)
	}
}
