//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTextPermissionOwnsCurrentAuthorityAndRemainingAllocation(t *testing.T) {
	window := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	profile := state.ClosedProfileView{NetworkID: fixtureID(1)}
	permission := &textPermission{profile: profile, accepted: credential.Permission{
		NotBefore: window, NotAfter: window.Add(time.Hour), Signature: [64]byte{1}, Maxima: [3]uint32{4, 2, 1},
	}, reserved: [3]uint32{3, 2, 2}}
	if !permission.currentFor(profile, window) || !permission.currentFor(profile, window.Add(time.Hour-time.Nanosecond)) ||
		permission.currentFor(profile, window.Add(-time.Nanosecond)) || permission.currentFor(profile, window.Add(time.Hour)) {
		t.Fatal("permission currentness did not respect its exact hour")
	}
	if permission.currentFor(state.ClosedProfileView{}, window) || (*textPermission)(nil).currentFor(profile, window) {
		t.Fatal("foreign or missing permission admitted")
	}
	permission.accepted.Signature = [64]byte{}
	if permission.currentFor(profile, window) {
		t.Fatal("unsigned permission admitted")
	}
	if permission.remaining(1) != 1 || permission.remaining(2) != 0 || permission.remaining(3) != 0 ||
		permission.remaining(0) != 0 || permission.remaining(4) != 0 || (*textPermission)(nil).remaining(1) != 0 {
		t.Fatal("permission allocation underflowed or accepted an invalid class")
	}
}

func TestTextPermissionRejectsEmptyOrInvalidClassBeforeBatchPreparation(t *testing.T) {
	permission := &textPermission{accepted: credential.Permission{Maxima: [3]uint32{1, 1, 1}}}
	for _, challenges := range [][]credential.ClosedTokenContext{
		nil,
		{{Class: 0}},
		{{Class: 4}},
	} {
		if batch, err := permission.reserveBatchLocked(state.ClosedProfileView{}, time.Now(), challenges,
			route.ClosedBootstrapSelection{}, false, nil, false, nil); err == nil || batch != nil {
			t.Fatalf("invalid batch admitted: batch=%v err=%v", batch, err)
		}
	}
}

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
