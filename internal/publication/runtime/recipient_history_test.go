package runtime

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"
)

// These tests isolate private accounting transitions. Calling commit here is
// not evidence of Connection acceptance, qualified Execution or wire delivery.
func accountingHistory() *recipientHistory {
	return &recipientHistory{ctx: context.Background(), entries: make(map[[32]byte]*openingReservation)}
}

func accountingNonce(number uint64) (nonce [32]byte) {
	binary.BigEndian.PutUint64(nonce[:8], number)
	return nonce
}

func TestRecipientHistoryConcurrentDuplicateHasOneReservation(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	var workers sync.WaitGroup
	accepted := make(chan *openingReservation, 16)
	for range 16 {
		workers.Go(func() {
			reservation, err := history.reserve(accountingNonce(1), now.Add(time.Second), now.Add(time.Minute), now)
			if err == nil {
				accepted <- reservation
			}
		})
	}
	workers.Wait()
	close(accepted)
	if len(accepted) != 1 {
		t.Fatalf("duplicate reservations: %d", len(accepted))
	}
	for reservation := range accepted {
		reservation.release()
		reservation.release()
	}
	history.users.Wait()
	if len(history.entries) != 0 {
		t.Fatal("unaccepted opening retained a replay entry")
	}
}

func TestRecipientHistoryRollingRateDoesNotRefundFailedOpening(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	for number := uint64(1); number <= 4; number++ {
		reservation, err := history.reserve(accountingNonce(number), now.Add(time.Minute), now.Add(time.Minute), now)
		if err != nil {
			t.Fatal(err)
		}
		reservation.release()
	}
	if _, err := history.reserve(accountingNonce(5), now.Add(time.Minute), now.Add(time.Minute), now.Add(time.Second-time.Nanosecond)); err == nil {
		t.Fatal("failed opening refunded rolling-second capacity")
	}
	reservation, err := history.reserve(accountingNonce(5), now.Add(time.Minute), now.Add(time.Minute), now.Add(time.Second))
	if err != nil {
		t.Fatal("exact rolling-second boundary refused", err)
	}
	reservation.release()
}

func TestRecipientHistoryAcceptedNonceUsesOriginalRegistrationHorizon(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	registrationEnd := now.Add(600 * time.Second)
	reservation, err := history.reserve(accountingNonce(1), now.Add(10*time.Second), registrationEnd, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.commit(now); err != nil {
		t.Fatal(err)
	}
	reservation.release()
	// Failure after private commit cannot roll back replay history. A later
	// registration can carry the same nonce but cannot accept it prematurely.
	horizon := registrationEnd.Add(60 * time.Second)
	if _, err := history.reserve(accountingNonce(1), horizon.Add(time.Minute), horizon.Add(time.Minute), horizon.Add(-time.Nanosecond)); err == nil {
		t.Fatal("accepted nonce retired before original expiry+60")
	}
	replacement, err := history.reserve(accountingNonce(1), horizon.Add(time.Minute), horizon.Add(time.Minute), horizon)
	if err != nil {
		t.Fatal("exact replay horizon did not release entry", err)
	}
	replacement.release()
}

func TestRecipientHistoryCapacityIncludesPendingAndAccepted(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	end := now.Add(2000 * time.Second)
	for number := uint64(1); number <= 2640; number++ {
		at := now.Add(time.Duration((number-1)/4) * time.Second)
		reservation, err := history.reserve(accountingNonce(number), end, end, at)
		if err != nil {
			t.Fatal(number, err)
		}
		if err := reservation.commit(at); err != nil {
			t.Fatal(err)
		}
		reservation.release()
	}
	if _, err := history.reserve(accountingNonce(2641), end, end, now.Add(661*time.Second)); err == nil {
		t.Fatal("full replay history accepted another nonce")
	}
}

func TestRecipientHistoryRefusesRevokedAndBackwardCommit(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	reservation, err := history.reserve(accountingNonce(1), now.Add(time.Minute), now.Add(time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	defer reservation.release()
	if reservation.commit(now.Add(-time.Nanosecond)) == nil {
		t.Fatal("backward clock committed a nonce")
	}
	ctx, cancel := context.WithCancel(context.Background())
	history.ctx = ctx
	cancel()
	if reservation.commit(now) == nil {
		t.Fatal("revoked history committed a nonce")
	}
	if _, err := history.reserve(accountingNonce(2), now.Add(time.Minute), now.Add(time.Minute), now); err == nil {
		t.Fatal("revoked history admitted another opening")
	}
}

func TestRecipientHistoryRollingRateUsesAttemptTimes(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	for i, offset := range []time.Duration{0, 200 * time.Millisecond, 400 * time.Millisecond, 600 * time.Millisecond} {
		at := now.Add(offset)
		reservation, err := history.reserve(accountingNonce(uint64(i+1)), now.Add(time.Minute), now.Add(time.Minute), at)
		if err != nil {
			t.Fatal(err)
		}
		reservation.release()
	}
	if _, err := history.reserve(accountingNonce(5), now.Add(time.Minute), now.Add(time.Minute), now.Add(999*time.Millisecond)); err == nil || err.Error() != "Publisher opening rate unavailable" {
		t.Fatal("rolling rate lost its original attempts", err)
	}
	reservation, err := history.reserve(accountingNonce(5), now.Add(time.Minute), now.Add(time.Minute), now.Add(time.Second))
	if err != nil {
		t.Fatal("oldest attempt did not expire at its exact boundary", err)
	}
	reservation.release()
	if _, err := history.reserve(accountingNonce(6), now.Add(time.Minute), now.Add(time.Minute), now.Add(1199*time.Millisecond)); err == nil || err.Error() != "Publisher opening rate unavailable" {
		t.Fatal("one expired attempt incorrectly reset the whole window", err)
	}
	reservation, err = history.reserve(accountingNonce(6), now.Add(time.Minute), now.Add(time.Minute), now.Add(1200*time.Millisecond))
	if err != nil {
		t.Fatal("second original attempt did not expire independently", err)
	}
	reservation.release()
}

func TestRecipientHistoryCapacityRetainsPendingUntilOriginalRelease(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	// Long accounting bounds isolate capacity from pruning. No signed
	// registration or network lifetime is authorized by this fixture.
	end := now.Add(2000 * time.Second)
	var pending *openingReservation
	for number := uint64(1); number <= 2640; number++ {
		at := now.Add(time.Duration((number-1)/4) * time.Second)
		reservation, err := history.reserve(accountingNonce(number), end, end, at)
		if err != nil {
			t.Fatal(number, err)
		}
		if number == 2640 {
			pending = reservation
			continue
		}
		if err := reservation.commit(at); err != nil {
			t.Fatal(err)
		}
		reservation.release()
	}
	at := now.Add(661 * time.Second)
	if _, err := history.reserve(accountingNonce(2641), end, end, at); err == nil || err.Error() != "Publisher replay capacity unavailable" {
		t.Fatal("pending opening bypassed shared replay capacity", err)
	}
	pending.release()
	pending.release()
	if err := pending.commit(at); err == nil {
		t.Fatal("released original opening committed after returning capacity")
	}
	replacement, err := history.reserve(accountingNonce(2641), end, end, at)
	if err != nil {
		t.Fatal("original pending release failed to return one slot", err)
	}
	defer replacement.release()
	if _, err := history.reserve(accountingNonce(2642), end, end, at); err == nil || err.Error() != "Publisher replay capacity unavailable" {
		t.Fatal("repeated release returned another capacity slot", err)
	}
}

func TestRecipientHistoryExpiredCommitDoesNotRetainNonce(t *testing.T) {
	history := accountingHistory()
	now := time.Unix(1000, 0)
	expiry := now.Add(time.Second)
	reservation, err := history.reserve(accountingNonce(1), expiry, now.Add(time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.commit(expiry); err == nil {
		t.Fatal("expired original opening accepted a nonce")
	}
	reservation.release()
	replacement, err := history.reserve(accountingNonce(1), now.Add(time.Minute), now.Add(time.Minute), expiry)
	if err != nil {
		t.Fatal("failed exact-boundary commit retained an accepted nonce", err)
	}
	replacement.release()
}
