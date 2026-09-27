package state

import (
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

func TestCompleteSourceWaveRechecksTrustedTimeBeforeActivatingPending(t *testing.T) {
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	beforeExpiry := time.Unix(1_800_000_001, 0).UTC()
	pending := epoch.Decision{Header: epoch.Header{Number: 2, Digest: [32]byte{2}, ValidFrom: beforeExpiry.Add(-time.Second), ValidUntil: beforeExpiry.Add(time.Second)}}
	current := &Snapshot{Epoch: 1, Digest: [32]byte{1}}
	opened := &networkState{config: config{root: root, localRoles: root + "-roles",
		clock: func() time.Time { return beforeExpiry }, observe: func() time.Time { return beforeExpiry },
		anchorWall: beforeExpiry, anchorMono: time.Now().Add(-1100 * time.Millisecond)}, storage: storage, current: current,
		pendingDecision: &pending}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	opened.distribution.pendingDigest = pending.Header.Digest
	opened.distribution.pendingValidFrom = pending.Header.ValidFrom.Unix()
	if _, err := opened.completeSourceWave(beforeExpiry, current, []sourceResult{{slot: 0, decision: pending, observations: [4]byte{sourceOutcomeValid}}}); err == nil || !strings.Contains(err.Error(), "expired before source wave completed") {
		t.Fatalf("late pending activation returned %v", err)
	}
	if opened.current == nil || opened.current.Digest != current.Digest || opened.pendingDecision == nil ||
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
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &Snapshot{Epoch: 1, Digest: [32]byte{1}}}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	first := epoch.Decision{Header: epoch.Header{Number: 2, Digest: [32]byte{2}}}
	second := epoch.Decision{Header: epoch.Header{Number: 2, Digest: [32]byte{3}}}
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
	pending := epoch.Decision{Header: epoch.Header{Number: 2, Digest: [32]byte{2}}}
	competing := epoch.Decision{Header: epoch.Header{Number: 2, Digest: [32]byte{3}}}
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &Snapshot{Epoch: 1, Digest: [32]byte{1}}, pendingDecision: &pending}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
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
