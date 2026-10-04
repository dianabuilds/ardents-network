package state

import (
	"strings"
	"testing"
	"time"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func TestCompleteSourceWaveRechecksTrustedTimeBeforeActivatingPending(t *testing.T) {
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	beforeExpiry := time.Unix(1_800_000_001, 0).UTC()
	pending := completionEpoch(2, 2, beforeExpiry)
	pending.Header.ValidUntil = beforeExpiry.Add(time.Second)
	predecessor := completionEpoch(1, 1, beforeExpiry)
	current := &predecessor
	opened := &networkState{config: config{root: root, localRoles: root + "-roles",
		clock: func() time.Time { return beforeExpiry }, observe: func() time.Time { return beforeExpiry },
		anchorWall: beforeExpiry, anchorMono: time.Now().Add(-1100 * time.Millisecond)}, storage: storage, current: current,
		pendingDecision: &pending}
	opened.distribution.epochFloor = opened.current.Snapshot.Epoch
	opened.distribution.epochDigest = opened.current.Snapshot.Digest
	opened.distribution.pendingDigest = pending.Header.Digest
	opened.distribution.pendingValidFrom = pending.Header.ValidFrom.Unix()
	if _, err := opened.completeSourceWave(beforeExpiry, current, []sourceResult{{slot: 0, decision: pending, observations: [4]byte{sourceOutcomeValid}}}); err == nil || !strings.Contains(err.Error(), "expired before source wave completed") {
		t.Fatalf("late pending activation returned %v", err)
	}
	if opened.current == nil || opened.current.Snapshot.Digest != current.Snapshot.Digest || opened.pendingDecision == nil ||
		opened.pendingDecision.Header.Digest != pending.Header.Digest {
		t.Fatalf("late completion changed current/pending state")
	}
}

func TestCompleteSourceWaveRecordsConflictBeforeCompletionClockFailure(t *testing.T) {
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	started := time.Unix(1_800_000_000, 0).UTC()
	predecessor := completionEpoch(1, 1, started)
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &predecessor}
	opened.distribution.epochFloor = opened.current.Snapshot.Epoch
	opened.distribution.epochDigest = opened.current.Snapshot.Digest
	first := completionEpoch(2, 2, started)
	second := completionEpoch(2, 3, started)
	if _, err := opened.completeSourceWave(started, opened.current, []sourceResult{
		{slot: 0, decision: first, observations: [4]byte{sourceOutcomeValid}},
		{slot: 1, decision: second, observations: [4]byte{0, sourceOutcomeValid}},
	}); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting wave returned %v", err)
	}
	if !opened.distribution.conflicting || opened.distribution.observedDigests[0] != first.Header.Digest ||
		opened.distribution.observedDigests[1] != second.Header.Digest {
		t.Fatalf("conflicting wave did not preserve observations")
	}
}

func TestCompleteSourceWaveRecordsPendingConflictBeforeCompletionClockFailure(t *testing.T) {
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	started := time.Unix(1_800_000_000, 0).UTC()
	pending := completionEpoch(2, 2, started)
	competing := completionEpoch(2, 3, started)
	predecessor := completionEpoch(1, 1, started)
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &predecessor, pendingDecision: &pending}
	opened.distribution.epochFloor = opened.current.Snapshot.Epoch
	opened.distribution.epochDigest = opened.current.Snapshot.Digest
	opened.distribution.pendingDigest = pending.Header.Digest
	if _, err := opened.completeSourceWave(started, opened.current, []sourceResult{
		{slot: 0, decision: competing, observations: [4]byte{sourceOutcomeValid}},
		{slot: 1, decision: competing, observations: [4]byte{0, sourceOutcomeValid}},
	}); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("pending conflict wave returned %v", err)
	}
	if !opened.distribution.conflicting || opened.distribution.observedDigests[0] != competing.Header.Digest ||
		opened.distribution.observedDigests[1] != competing.Header.Digest {
		t.Fatalf("pending conflict wave did not preserve observations")
	}
}

// These tests isolate completion scheduling after authentication. Supply a
// complete chain identity; signature verification is exercised by Source tests.
func completionEpoch(number uint64, digest byte, now time.Time) epoch2.Decision {
	header := epoch2.Header{NetworkID: [32]byte{9}, Number: number, Digest: [32]byte{digest},
		ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Hour)}
	if number > 1 {
		header.Previous = [32]byte{1}
	}
	return epoch2.Decision{Header: header, Snapshot: epoch2.Snapshot{Epoch: number, Digest: header.Digest}}
}
