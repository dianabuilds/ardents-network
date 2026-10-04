package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func TestSourceCycleClosurePersistsMissingResultAsInterrupted(t *testing.T) {
	now := time.Unix(1_800_000_100, 0).UTC()
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	configuration := config{root: root, localRoles: filepath.Join(t.TempDir(), "roles"), clock: func() time.Time { return now },
		sourceInfo: source.Details{Configured: true, OrderSeed: [32]byte{1}, Identities: [2][32]byte{{1}, {2}},
			Exposures: [2][32]byte{{3}, {4}}, Families: [2]string{"source-one", "source-two"}}}
	owner := &networkState{config: configuration, storage: storage}
	if err := owner.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	order, deadline, err := owner.startSourceWave(now)
	if err != nil {
		t.Fatal(err)
	}
	contact, _, err := owner.beginLatestAttempt(order[0])
	if err != nil || !contact {
		t.Fatalf("contact proposal = %t (%v)", contact, err)
	}
	if err := owner.commitSourceFailure(now.Add(time.Second), [4]byte{}, [4]uint64{}, [4][32]byte{}); err != nil {
		t.Fatal(err)
	}
	if owner.distribution.outcomes[order[0]] != sourceOutcomeInterrupted || owner.distribution.consecutiveFailures != 1 || owner.distribution.cycleActive {
		t.Fatal("closure erased the consumed attempt or cleared backoff")
	}
	if err := owner.releaseSourceWaveLocked(); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedStorage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedStorage.Close()
	reopened := &networkState{config: configuration, storage: reopenedStorage}
	if err := reopened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	if reopened.distribution.outcomes[order[0]] != sourceOutcomeInterrupted || reopened.distribution.attempts[order[0]] != sourceAttemptFailed || reopened.distribution.cycleDeadline != deadline.Unix() {
		t.Fatal("reopening lost interruption or original deadline")
	}
	if _, _, err := reopened.startSourceWave(now.Add(2 * time.Second)); err == nil {
		t.Fatal("reopening bypassed durable backoff")
	}
}
