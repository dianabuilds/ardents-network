//go:build linux

package node

import (
	"testing"
	"time"
)

func TestPrivateRecipientFixturePreservesItsCompleteTestWindow(t *testing.T) {
	for _, minute := range []int{0, 58, 59} {
		for _, second := range []int{0, 15, 59} {
			now := time.Date(2026, 9, 13, 23, minute, second, 0, time.UTC)
			start, authorityEnd := privateRecipientFixtureWindow(now)
			// These live-network tests include a 45-second child-process deadline,
			// and request registration lasting up to 60 seconds after initial setup.
			if start.Before(now) || start.Sub(now) > privateRecipientFixtureMinimumWindow || authorityEnd.Sub(start) < privateRecipientFixtureMinimumWindow {
				t.Fatalf("test can outlive its redemption window at %v: start %v", now, start)
			}
			if !authorityEnd.Equal(start.Truncate(time.Hour).Add(time.Hour)) {
				t.Fatalf("registration cannot retain its declared authority at %v: %v", start, authorityEnd)
			}
		}
	}
}
