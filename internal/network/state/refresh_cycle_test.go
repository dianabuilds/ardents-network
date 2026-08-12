package state

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	statestore "github.com/dianabuilds/ardents-network/internal/network/store"
)

func TestInterruptedCycleResumesOnlyUnstartedLatestAttempt(t *testing.T) {
	root := t.TempDir()
	storage, err := statestore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, _, err := source.New(source.Config{OrderSeed: sha256.Sum256([]byte("resume-order"))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	config := config{root: root, source: plan}
	initial := &store{config: config, storage: storage}
	if err := initial.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_100, 0).UTC()
	order, deadline, err := initial.startSourceWave(now)
	if err != nil {
		t.Fatal(err)
	}
	cycle := initial.distribution
	cycle.sequence++
	cycle.attempts[order[0]] = 1
	if err := initial.commitDistribution(cycle); err != nil {
		t.Fatalf("record first LATEST: %v", err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	storage, err = statestore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	restarted := &store{config: config, storage: storage}
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
	if restarted.distribution.attempts[order[1]] != 0 {
		t.Fatal("unstarted LATEST attempt was consumed during recovery")
	}
}
