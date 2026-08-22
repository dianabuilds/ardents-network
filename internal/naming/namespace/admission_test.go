package namespace_test

import (
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestAdmissionVerifyRejectsReplayAndScopeMutation(t *testing.T) {
	t.Parallel()
	gate := admissionGate(t)
	proof := admissionProof(t, gate, "resolution", 1)
	if ok, reason := gate.Verify(950, proof); !ok || reason != "" {
		t.Fatalf("first verification ok=%v reason=%q", ok, reason)
	}
	if ok, reason := gate.Verify(950, proof); ok || reason != "replay" {
		t.Fatalf("replay ok=%v reason=%q", ok, reason)
	}
	mutated := admissionProof(t, gate, "resolution", 2)
	mutated.Challenge.IsolationBinding[0]++
	if ok, _ := gate.Verify(950, mutated); ok {
		t.Fatal("cross-Isolation proof was admitted")
	}
}

func TestAdmissionVerifyAdmitsOneParallelProofWithoutQueue(t *testing.T) {
	t.Parallel()
	gate := admissionGate(t)
	proof := admissionProof(t, gate, "resolution", 3)
	const attempts = 32
	results := make(chan bool, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ok, _ := gate.Verify(950, proof)
			results <- ok
		}()
	}
	workers.Wait()
	close(results)
	accepted := 0
	for result := range results {
		if result {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted=%d want=1", accepted)
	}
}

func TestAdmissionVerifyFreezesEverySurfaceAndFailsClosedAcrossRestart(t *testing.T) {
	t.Parallel()
	expected := map[string]uint8{"resolution": 16, "renewal-update": 16, "policy-recovery": 17, "root-claim": 18}
	for surface, bits := range expected {
		gate := admissionGate(t)
		proof := admissionProof(t, gate, surface, byte(bits))
		if proof.Challenge.WorkBits != bits {
			t.Fatalf("%s bits=%d want=%d", surface, proof.Challenge.WorkBits, bits)
		}
		if ok, reason := gate.Verify(proof.Challenge.ExpiresAt, proof); ok || reason != "invalid-scope" {
			t.Fatalf("%s expired proof ok=%v reason=%q", surface, ok, reason)
		}
	}
	old := admissionGate(t)
	proof := admissionProof(t, old, "root-claim", 42)
	restarted, err := namespace.NewAdmission([32]byte{1}, [32]byte{2}, 3, [32]byte{9})
	if err != nil {
		t.Fatal(err)
	}
	if ok, reason := restarted.Verify(950, proof); ok || reason != "invalid-challenge" {
		t.Fatalf("pre-restart proof ok=%v reason=%q", ok, reason)
	}
}

func admissionGate(t testing.TB) *namespace.Admission {
	t.Helper()
	gate, err := namespace.NewAdmission([32]byte{1}, [32]byte{2}, 3, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	return gate
}

func admissionProof(t testing.TB, gate *namespace.Admission, surface string, nonce byte) namespace.Proof {
	t.Helper()
	challenge, err := gate.Issue(900, surface, sha256.Sum256([]byte{nonce, 1}),
		sha256.Sum256([]byte{nonce, 2}), 1_000, [16]byte{nonce})
	if err != nil {
		t.Fatal(err)
	}
	if !challenge.BindsIsolation(sha256.Sum256([]byte{nonce, 2})) {
		t.Fatal("challenge lost its local Isolation Context binding")
	}
	proof, _ := challenge.Solve()
	return proof
}
