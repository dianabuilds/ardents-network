package state

import (
	"strings"
	"testing"
	"time"
)

func TestCompleteSourceWaveRechecksTrustedTimeBeforeActivatingPending(t *testing.T) {
	root := t.TempDir()
	storage, err := openDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.close()
	beforeExpiry := time.Unix(1_800_000_001, 0).UTC()
	pending := candidateDecision{epoch: epochEnvelope{number: 2, digest: [32]byte{2}, validFrom: beforeExpiry.Add(-time.Second), validUntil: beforeExpiry.Add(time.Second)}}
	current := &Snapshot{Epoch: 1, Digest: [32]byte{1}}
	opened := &networkState{config: config{root: root, localRoles: root + "-roles",
		clock: func() time.Time { return beforeExpiry }, observe: func() time.Time { return beforeExpiry },
		anchorWall: beforeExpiry, anchorMono: time.Now().Add(-1100 * time.Millisecond)}, storage: storage, current: current,
		pendingDecision: &pending}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	opened.distribution.pendingDigest = pending.epoch.digest
	opened.distribution.pendingValidFrom = pending.epoch.validFrom.Unix()
	if _, err := opened.completeSourceWave(beforeExpiry, current, []sourceResult{{slot: 0, decision: pending, observations: [4]byte{sourceOutcomeValid}}}); err == nil || !strings.Contains(err.Error(), "expired before source wave completed") {
		t.Fatalf("late pending activation returned %v", err)
	}
	if opened.current == nil || opened.current.Digest != current.Digest || opened.pendingDecision == nil ||
		opened.pendingDecision.epoch.digest != pending.epoch.digest {
		t.Fatalf("late completion changed current/pending state")
	}
}

func TestCompleteSourceWaveRecordsConflictBeforeCompletionClockFailure(t *testing.T) {
	root := t.TempDir()
	storage, err := openDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.close()
	started := time.Unix(1_800_000_000, 0).UTC()
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &Snapshot{Epoch: 1, Digest: [32]byte{1}}}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	first := candidateDecision{epoch: epochEnvelope{number: 2, digest: [32]byte{2}}}
	second := candidateDecision{epoch: epochEnvelope{number: 2, digest: [32]byte{3}}}
	if _, err := opened.completeSourceWave(started, opened.current, []sourceResult{
		{slot: 0, decision: first, observations: [4]byte{sourceOutcomeValid}},
		{slot: 1, decision: second, observations: [4]byte{0, sourceOutcomeValid}},
	}); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting wave returned %v", err)
	}
	if !opened.distribution.conflicting || opened.distribution.observedDigests[0] != first.epoch.digest ||
		opened.distribution.observedDigests[1] != second.epoch.digest {
		t.Fatalf("conflicting wave did not preserve observations")
	}
}

func TestCompleteSourceWaveRecordsPendingConflictBeforeCompletionClockFailure(t *testing.T) {
	root := t.TempDir()
	storage, err := openDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.close()
	started := time.Unix(1_800_000_000, 0).UTC()
	pending := candidateDecision{epoch: epochEnvelope{number: 2, digest: [32]byte{2}}}
	competing := candidateDecision{epoch: epochEnvelope{number: 2, digest: [32]byte{3}}}
	opened := &networkState{config: config{root: root, clock: func() time.Time { return started }, observe: func() time.Time { return time.Time{} }},
		storage: storage, current: &Snapshot{Epoch: 1, Digest: [32]byte{1}}, pendingDecision: &pending}
	if err := opened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	opened.distribution.pendingDigest = pending.epoch.digest
	if _, err := opened.completeSourceWave(started, opened.current, []sourceResult{
		{slot: 0, decision: competing, observations: [4]byte{sourceOutcomeValid}},
		{slot: 1, decision: competing, observations: [4]byte{0, sourceOutcomeValid}},
	}); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("pending conflict wave returned %v", err)
	}
	if !opened.distribution.conflicting || opened.distribution.observedDigests[0] != competing.epoch.digest ||
		opened.distribution.observedDigests[1] != competing.epoch.digest {
		t.Fatalf("pending conflict wave did not preserve observations")
	}
}
