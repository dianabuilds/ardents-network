//go:build ignore

package main

import (
	"crypto/sha256"
	"sync"
	"testing"
)

func TestAdmissionVerifyAcceptsOneScopedProof(t *testing.T) {
	t.Parallel()
	gate, err := NewAdmission(Config{Node: [32]byte{1}, Network: [32]byte{2}, Epoch: 3,
		BootSecret: [32]byte{4}, MaxTTLMillis: 30_000,
		Profiles: []Profile{{Surface: Resolution, WorkBits: 8, MaxSpent: 16, MaxInFlight: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := gate.Issue(900, Request{Surface: Resolution,
		OperationDigest: sha256.Sum256([]byte("resolve alice")), IsolationContext: [32]byte{5},
		ExpiresAt: 1_000, Nonce: [16]byte{6}})
	if err != nil {
		t.Fatal(err)
	}
	proof, hashes := Solve(challenge)
	if hashes == 0 {
		t.Fatal("solver performed no work")
	}
	result := gate.Verify(950, proof)
	if result.Outcome != Admitted {
		t.Fatalf("result = %+v", result)
	}
}

func TestAdmissionVerifyRejectsReplayAndScopeMutations(t *testing.T) {
	t.Parallel()
	base := testAdmissionConfig()
	gate, err := NewAdmission(base)
	if err != nil {
		t.Fatal(err)
	}
	proof := testProof(t, gate, Resolution, 1)
	if got := gate.Verify(950, proof); got.Outcome != Admitted {
		t.Fatalf("first result = %+v", got)
	}
	if got := gate.Verify(950, proof); got.Reason != "replay" {
		t.Fatalf("replay result = %+v", got)
	}

	mutations := map[string]func(*Proof){
		"node":      func(p *Proof) { p.Challenge.Node[0]++ },
		"network":   func(p *Proof) { p.Challenge.Network[0]++ },
		"epoch":     func(p *Proof) { p.Challenge.Epoch++ },
		"surface":   func(p *Proof) { p.Challenge.Surface = Update },
		"operation": func(p *Proof) { p.Challenge.OperationDigest[0]++ },
		"isolation": func(p *Proof) { p.Challenge.IsolationContext[0]++ },
		"expiry":    func(p *Proof) { p.Challenge.ExpiresAt++ },
		"work bits": func(p *Proof) { p.Challenge.WorkBits++ },
		"tag":       func(p *Proof) { p.Challenge.AuthenticationTag[0]++ },
		"work nonce": func(p *Proof) {
			for p.WorkNonce++; validWork(p.Challenge, p.WorkNonce); p.WorkNonce++ {
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := testProof(t, gate, Resolution, byte(len(name)+20))
			mutate(&candidate)
			if got := gate.Verify(950, candidate); got.Outcome != AdmissionDenied {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestAdmissionVerifyBindsBootAndPrunesPerSurfaceState(t *testing.T) {
	t.Parallel()
	config := testAdmissionConfig()
	config.Profiles = []Profile{
		{Surface: Resolution, WorkBits: 8, MaxSpent: 1, MaxInFlight: 4},
		{Surface: Claim, WorkBits: 8, MaxSpent: 1, MaxInFlight: 4},
	}
	gate, err := NewAdmission(config)
	if err != nil {
		t.Fatal(err)
	}
	first := testProofAt(t, gate, Resolution, 1, 900, 1_000)
	if got := gate.Verify(950, first); got.Outcome != Admitted {
		t.Fatalf("first result = %+v", got)
	}
	if got := gate.Verify(950, testProof(t, gate, Resolution, 2)); got.Reason != "capacity" {
		t.Fatalf("resolution capacity = %+v", got)
	}
	if got := gate.Verify(950, testProof(t, gate, Claim, 3)); got.Outcome != Admitted {
		t.Fatalf("claim isolation = %+v", got)
	}
	if got := gate.Verify(1_050, testProofAt(t, gate, Resolution, 4, 1_010, 1_100)); got.Outcome != Admitted {
		t.Fatalf("pruned result = %+v", got)
	}

	restarted := config
	restarted.BootSecret[0]++
	newGate, err := NewAdmission(restarted)
	if err != nil {
		t.Fatal(err)
	}
	oldProof := testProofAt(t, gate, Resolution, 5, 1_010, 1_100)
	if got := newGate.Verify(1_050, oldProof); got.Outcome != AdmissionDenied {
		t.Fatalf("restart result = %+v", got)
	}
}

func TestAdmissionVerifyAdmitsParallelProofOnce(t *testing.T) {
	t.Parallel()
	gate, err := NewAdmission(testAdmissionConfig())
	if err != nil {
		t.Fatal(err)
	}
	proof := testProof(t, gate, Resolution, 9)
	const attempts = 32
	results := make(chan Result, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Add(1)
		go func() {
			defer workers.Done()
			results <- gate.Verify(950, proof)
		}()
	}
	workers.Wait()
	close(results)
	admitted := 0
	for result := range results {
		if result.Outcome == Admitted {
			admitted++
		} else if result.Reason != "replay" && result.Reason != "busy" {
			t.Fatalf("losing result = %+v", result)
		}
	}
	if admitted != 1 {
		t.Fatalf("admitted = %d, want 1", admitted)
	}
}

func TestAdmissionVerifyRejectsWithoutQueueWhenSurfaceIsBusy(t *testing.T) {
	t.Parallel()
	config := testAdmissionConfig()
	config.Profiles[0].MaxInFlight = 1
	gate, err := NewAdmission(config)
	if err != nil {
		t.Fatal(err)
	}
	gate.inflight[Resolution] <- struct{}{}
	result := gate.Verify(950, testProof(t, gate, Resolution, 10))
	<-gate.inflight[Resolution]
	if result.Reason != "busy" {
		t.Fatalf("result = %+v", result)
	}
}

func TestAdmissionVerifyDoesNotSpendCapacityForInvalidWork(t *testing.T) {
	t.Parallel()
	config := testAdmissionConfig()
	config.Profiles[0].MaxSpent = 1
	gate, err := NewAdmission(config)
	if err != nil {
		t.Fatal(err)
	}
	invalid := testProof(t, gate, Resolution, 11)
	for invalid.WorkNonce++; validWork(invalid.Challenge, invalid.WorkNonce); invalid.WorkNonce++ {
	}
	if got := gate.Verify(950, invalid); got.Reason != "insufficient-work" {
		t.Fatalf("invalid result = %+v", got)
	}
	if got := gate.Verify(950, testProof(t, gate, Resolution, 12)); got.Outcome != Admitted {
		t.Fatalf("valid result after rejection = %+v", got)
	}
}

func testAdmissionConfig() Config {
	return Config{Node: [32]byte{1}, Network: [32]byte{2}, Epoch: 3,
		BootSecret: [32]byte{4}, MaxTTLMillis: 30_000,
		Profiles: []Profile{{Surface: Resolution, WorkBits: 8, MaxSpent: 16, MaxInFlight: 4},
			{Surface: Update, WorkBits: 9, MaxSpent: 16, MaxInFlight: 4}}}
}

func testProof(t *testing.T, gate *Admission, surface Surface, nonce byte) Proof {
	t.Helper()
	return testProofAt(t, gate, surface, nonce, 900, 1_000)
}

func testProofAt(t *testing.T, gate *Admission, surface Surface, nonce byte, issued, expires int64) Proof {
	t.Helper()
	challenge, err := gate.Issue(issued, Request{Surface: surface,
		OperationDigest: sha256.Sum256([]byte{nonce, 1}), IsolationContext: sha256.Sum256([]byte{nonce, 2}),
		ExpiresAt: expires, Nonce: [16]byte{nonce}})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := Solve(challenge)
	return proof
}
