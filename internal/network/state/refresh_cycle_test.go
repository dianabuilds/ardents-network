package state

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func TestSourceCycleInterruptedAttemptRecovery(t *testing.T) {
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	config := config{root: root, sourceInfo: source.Details{OrderSeed: sha256.Sum256([]byte("resume-order"))},
		localRoles: root + "-local-roles", clock: func() time.Time { return time.Unix(1_800_000_100, 0).UTC() }}
	config.sourceInfo.Identities = [2][32]byte{{1}, {2}}
	config.sourceInfo.Exposures = [2][32]byte{{3}, {4}}
	config.sourceInfo.Families = [2]string{"source-family-one", "source-family-two"}
	initial := &networkState{config: config, storage: storage}
	if err := initial.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_100, 0).UTC()
	order, deadline, err := initial.startSourceWave(now)
	if err != nil {
		t.Fatal(err)
	}
	if started, outcome, err := initial.beginLatestAttempt(order[0]); err != nil || !started || outcome != 0 {
		t.Fatalf("record first LATEST: started=%t outcome=%d err=%v", started, outcome, err)
	}
	// The BY_DIGEST transport response finished, but the wave had not yet
	// verified its object or recorded an outcome when the process stopped.
	digestSlot := digestAttemptSlot(order[1])
	if err := initial.beginDigestAttempt(order[1], [32]byte{9}); err != nil {
		t.Fatal(err)
	}
	if err := initial.finishDigestAttempt(order[1], true); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	storage, err = openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	restarted := &networkState{config: config, storage: storage}
	if err := restarted.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	recoveredOrder, recoveredDeadline, err := restarted.startSourceWave(now.Add(time.Second))
	if err != nil || recoveredOrder != order || !recoveredDeadline.Equal(deadline) {
		t.Fatalf("resume order=%v deadline=%v err=%v", recoveredOrder, recoveredDeadline, err)
	}
	if started, outcome, err := restarted.beginLatestAttempt(order[0]); err != nil || started || outcome != sourceOutcomeInterrupted {
		t.Fatalf("repeated started attempt: started=%t outcome=%d err=%v", started, outcome, err)
	}
	if restarted.distribution.attempts[order[1]] != sourceAttemptNotStarted {
		t.Fatal("unstarted LATEST attempt was consumed during recovery")
	}
	if restarted.distribution.attempts[digestSlot] != sourceAttemptFailed ||
		restarted.distribution.outcomes[digestSlot] != sourceOutcomeInterrupted {
		t.Fatal("completed but unverified BY_DIGEST attempt has no interrupted recovery outcome")
	}

	// The deadline path must resolve the same incomplete persisted stage.
	expired := restarted.distribution
	expired.sequence++
	expired.attempts[digestSlot] = sourceAttemptCompleted
	expired.outcomes[digestSlot] = 0
	if err := restarted.commitDistribution(expired); err != nil {
		t.Fatal(err)
	}
	if _, _, err := restarted.startSourceWave(deadline.Add(time.Second)); err == nil {
		t.Fatal("expired cycle was resumed")
	}
	if restarted.distribution.cycleActive || restarted.distribution.attempts[digestSlot] != sourceAttemptFailed ||
		restarted.distribution.outcomes[digestSlot] != sourceOutcomeInterrupted {
		t.Fatal("expired cycle left completed BY_DIGEST without an interrupted outcome")
	}
}
