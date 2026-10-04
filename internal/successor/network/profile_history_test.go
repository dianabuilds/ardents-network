package network_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestProfileHistoryNeverSelectsWinnerAfterConflict(t *testing.T) {
	profile := membershipProfile(time.Unix(1800000000, 0).UTC())
	history, err := network.RestoreProfileHistory(profile.Generation, profile.Epoch, [32]byte{}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := history.Consider(profile)
	if err != nil || !first.NeedsCommit() || first.Conflicted() || first.AcceptedDigest() != profile.Digest {
		t.Fatalf("first acceptance: %v", err)
	}
	history, err = network.RestoreProfileHistory(profile.Generation, profile.Epoch, first.AcceptedDigest(), first.ConflictDigest())
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := history.Consider(profile)
	if err != nil || repeat.NeedsCommit() || repeat.Conflicted() {
		t.Fatalf("exact repeat changed history: %v", err)
	}
	second := profile
	second.Digest = [32]byte{99}
	conflict, err := history.Consider(second)
	if err != nil || !conflict.NeedsCommit() || !conflict.Conflicted() || conflict.AcceptedDigest() != profile.Digest || conflict.ConflictDigest() != second.Digest {
		t.Fatalf("conflict lost evidence: %v", err)
	}
	history, err = network.RestoreProfileHistory(profile.Generation, profile.Epoch, conflict.AcceptedDigest(), conflict.ConflictDigest())
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []network.ProfileBinding{profile, second} {
		decision, err := history.Consider(candidate)
		if err != nil || decision.NeedsCommit() || !decision.Conflicted() {
			t.Fatalf("reopen or repeat erased conflict: %v", err)
		}
	}
	second.Epoch++
	if _, err := history.Consider(second); err == nil {
		t.Fatal("another Epoch changed profile history")
	}
}
