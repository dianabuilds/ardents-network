//go:build linux

package tokens

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func fixtureID(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}

func TestTextPermissionOwnsCurrentAuthorityAndRemainingAllocation(t *testing.T) {
	window := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	profile := state.ClosedProfileView{NetworkID: fixtureID(1)}
	subject := &Permission{Profile: profile, Accepted: admission.Permission{
		NotBefore: window, NotAfter: window.Add(time.Hour), Signature: [64]byte{1}, Maxima: [3]uint32{4, 2, 1},
	}, Reserved: [3]uint32{3, 2, 2}}
	if !subject.CurrentFor(profile, window) || !subject.CurrentFor(profile, window.Add(time.Hour-time.Nanosecond)) ||
		subject.CurrentFor(profile, window.Add(-time.Nanosecond)) || subject.CurrentFor(profile, window.Add(time.Hour)) {
		t.Fatal("permission currentness did not respect its exact hour")
	}
	if subject.CurrentFor(state.ClosedProfileView{}, window) || (*Permission)(nil).CurrentFor(profile, window) {
		t.Fatal("foreign or missing permission admitted")
	}
	subject.Accepted.Signature = [64]byte{}
	if subject.CurrentFor(profile, window) {
		t.Fatal("unsigned permission admitted")
	}
	if subject.Remaining(1) != 1 || subject.Remaining(2) != 0 || subject.Remaining(3) != 0 ||
		subject.Remaining(0) != 0 || subject.Remaining(4) != 0 || (*Permission)(nil).Remaining(1) != 0 {
		t.Fatal("permission allocation underflowed or accepted an invalid class")
	}
}

func TestTextPermissionRejectsEmptyOrInvalidClassBeforeBatchPreparation(t *testing.T) {
	subject := &Permission{Accepted: admission.Permission{Maxima: [3]uint32{1, 1, 1}}}
	for _, challenges := range [][]credential.ClosedTokenContext{
		nil,
		{{Class: 0}},
		{{Class: 4}},
	} {
		if batch, err := subject.ReserveBatch(state.ClosedProfileView{}, time.Now(), challenges,
			client.ClosedBootstrapSelection{}, false, nil, false, nil); err == nil || batch != nil {
			t.Fatalf("invalid batch admitted: batch=%v err=%v", batch, err)
		}
	}
}

func TestTextPermissionStockPreflightSeparatesWindowClassAndKnownDuty(t *testing.T) {
	window := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	profile, receiver := fixtureID(1), fixtureID(2)
	stock := func(digest, node [32]byte, duty uint64, class uint8, at time.Time, count int) Stock {
		return Stock{
			Challenge: credential.ClosedTokenContext{
				ProfileDigest: digest, ReceiverNodeID: node, ReceiverDutyGeneration: duty,
				Class: class, WindowStart: at,
			},
			Tokens: make([][]byte, count),
		}
	}
	subject := &Permission{
		Accepted: admission.Permission{NotBefore: window},
		Stock: []Stock{
			stock(profile, receiver, 7, 2, window, 2),
			stock(profile, receiver, 8, 2, window, 1),
			stock(profile, receiver, 7, 1, window, 1),
			stock(profile, receiver, 7, 2, window.Add(-time.Hour), 1),
			stock(fixtureID(3), receiver, 7, 2, window, 1),
			stock(profile, fixtureID(4), 7, 2, window, 1),
		},
	}
	if got := subject.StockCountFor(profile, receiver, 2); got != 3 {
		t.Fatalf("candidate stock count = %d, want 3", got)
	}
	if got := subject.StockCountForDuty(profile, receiver, 7, 2); got != 2 {
		t.Fatalf("exact duty stock count = %d, want 2", got)
	}
	if got := subject.StockCountForDuty(profile, receiver, 9, 2); got != 0 {
		t.Fatalf("foreign duty stock count = %d, want 0", got)
	}
}
