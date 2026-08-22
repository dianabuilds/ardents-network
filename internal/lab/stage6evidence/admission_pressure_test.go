package stage6evidence

import (
	"crypto/sha256"
	"runtime"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestMeasureAdmissionBusyCreatesARealOverlap(t *testing.T) {
	evidence := admissionCellEvidence{Node: [32]byte{1}, Network: [32]byte{2}, Epoch: 3,
		Now: 950, Isolation: sha256.Sum256([]byte("pressure-test-isolation"))}
	gate, err := namespace.NewAdmission(evidence.Node, evidence.Network, evidence.Epoch, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	const maximumSpent = 2048
	proofs := make([]namespace.Proof, maximumSpent)
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for ordinal := range jobs {
				challenge, issueErr := pressureChallenge(gate, evidence, 1, "renewal-update", ordinal)
				if issueErr == nil {
					proofs[ordinal], _ = challenge.Solve()
				}
			}
		}()
	}
	for ordinal := range proofs {
		jobs <- ordinal
	}
	close(jobs)
	workers.Wait()
	for _, proof := range proofs {
		if ok, reason := gate.Verify(evidence.Now, proof); !ok || reason != "" {
			t.Fatalf("saturating proof was rejected: %q", reason)
		}
	}
	overflowChallenge, err := pressureChallenge(gate, evidence, 1, "renewal-update", maximumSpent)
	if err != nil {
		t.Fatal(err)
	}
	overflow := namespace.Proof{Challenge: overflowChallenge}
	if ok, reason := gate.Verify(evidence.Now, overflow); ok || reason != "capacity" {
		t.Fatalf("overflow proof ok=%v reason=%q", ok, reason)
	}

	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	var capacity admissionCapacityEvidence
	busyChallenge, err := pressureChallengeUntil(gate, evidence, 1, "renewal-update", maximumSpent+1, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if err := measureAdmissionBusy([32]byte{4}, evidence, proofs,
		namespace.Proof{Challenge: busyChallenge}, 32, &capacity); err != nil {
		t.Fatal(err)
	}
	if !containsAdmissionOutcome(capacity.BusyOutcomes, "busy") ||
		!containsAdmissionOutcome(capacity.BusyOutcomes, "insufficient-work") {
		t.Fatal("busy limit was not observed under concurrent pressure")
	}
}
