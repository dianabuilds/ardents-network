//go:build linux

package node

import (
	"testing"
	"time"
)

const privateRecipientFixtureMinimumWindow = 2 * time.Minute

func privateRecipientFixtureWindow(now time.Time) (time.Time, time.Time) {
	start := now
	if now.Truncate(time.Hour).Add(time.Hour).Sub(now) <= privateRecipientFixtureMinimumWindow {
		start = now.Truncate(time.Hour).Add(time.Hour)
	}
	return start, start.Truncate(time.Hour).Add(time.Hour)
}

func privateRecipientFixtureStart(t *testing.T) (time.Time, time.Time) {
	t.Helper()
	for {
		now := time.Now().UTC()
		start, end := privateRecipientFixtureWindow(now)
		if !start.After(now) {
			return now.Truncate(time.Second), end
		}
		// Wait for a valid test issuance window; do not change product clocks,
		// redeem a future token early, or drop the test as an unavailable skip.
		timer := time.NewTimer(time.Until(start))
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal("recipient fixture window canceled")
		case <-timer.C:
		}
	}
}
