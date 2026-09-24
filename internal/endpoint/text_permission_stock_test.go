//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTextPermissionStockPreflightSeparatesWindowClassAndKnownDuty(t *testing.T) {
	window := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	profile, receiver := fixtureID(1), fixtureID(2)
	stock := func(digest, node [32]byte, duty uint64, class uint8, at time.Time, count int) textTokenStock {
		return textTokenStock{
			challenge: credential.ClosedTokenContext{
				ProfileDigest: digest, ReceiverNodeID: node, ReceiverDutyGeneration: duty,
				Class: class, WindowStart: at,
			},
			tokens: make([][]byte, count),
		}
	}
	permission := &textPermission{
		accepted: credential.Permission{NotBefore: window},
		stock: []textTokenStock{
			stock(profile, receiver, 7, 2, window, 2),
			stock(profile, receiver, 8, 2, window, 1),
			stock(profile, receiver, 7, 1, window, 1),
			stock(profile, receiver, 7, 2, window.Add(-time.Hour), 1),
			stock(fixtureID(3), receiver, 7, 2, window, 1),
			stock(profile, fixtureID(4), 7, 2, window, 1),
		},
	}
	if got := permission.stockCountFor(profile, receiver, 2); got != 3 {
		t.Fatalf("candidate stock count = %d, want 3", got)
	}
	if got := permission.stockCountForDuty(profile, receiver, 7, 2); got != 2 {
		t.Fatalf("exact duty stock count = %d, want 2", got)
	}
	if got := permission.stockCountForDuty(profile, receiver, 9, 2); got != 0 {
		t.Fatalf("foreign duty stock count = %d, want 0", got)
	}
}
