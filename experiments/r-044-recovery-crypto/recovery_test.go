//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestAuthorizeAcceptsThresholdOfDistinctRecoveryAuthorities(t *testing.T) {
	t.Parallel()
	policy, signers := recoveryFixture(t, 2, 3)
	proof := signedRecoveryProof(t, policy, signers[:2], Initiate)

	result := Authorize(policy, proof)
	if result.Outcome != Authorized || result.ValidSigners != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAuthorizeRejectsPolicyWithoutVisibleMinimumDelay(t *testing.T) {
	t.Parallel()
	policy, signers := recoveryFixture(t, 2, 3)
	policy.Delay = time.Hour
	proof := signedRecoveryProof(t, policy, signers[:2], Initiate)
	if result := Authorize(policy, proof); result.Outcome != Denied || result.Reason != "invalid-policy" {
		t.Fatalf("result = %+v", result)
	}
}

func TestAuthorizeHostileRecoveryMatrix(t *testing.T) {
	policy, signers := recoveryFixture(t, 2, 3)
	valid := signedRecoveryProof(t, policy, signers[:2], Initiate)

	tests := []struct {
		name   string
		mutate func(*Policy, *Proof)
	}{
		{"t-minus-one", func(_ *Policy, p *Proof) { p.Signatures = p.Signatures[:1] }},
		{"duplicate-signer", func(_ *Policy, p *Proof) { p.Signatures[1] = p.Signatures[0] }},
		{"unknown-signer", func(_ *Policy, p *Proof) { p.Signatures[0].Signer = [32]byte{99} }},
		{"wrong-generation", func(p *Policy, _ *Proof) { p.Generation++ }},
		{"wrong-policy-revision", func(p *Policy, _ *Proof) { p.Revision++ }},
		{"wrong-name", func(p *Policy, _ *Proof) { p.Name = "bob" }},
		{"wrong-network", func(p *Policy, _ *Proof) { p.Network[0] ^= 0xff }},
		{"changed-successor", func(_ *Policy, p *Proof) { p.Successor[0] ^= 0xff }},
		{"changed-deadline", func(_ *Policy, p *Proof) { p.CompletesAt++ }},
		{"initiate-signature-as-cancel", func(_ *Policy, p *Proof) { p.Operation = Cancel }},
		{"malformed-signature", func(_ *Policy, p *Proof) { p.Signatures[0].Bytes = []byte{1} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changedPolicy := clonePolicy(policy)
			changedProof := cloneProof(valid)
			test.mutate(&changedPolicy, &changedProof)
			beforePolicy, beforeProof := clonePolicy(changedPolicy), cloneProof(changedProof)
			if result := Authorize(changedPolicy, changedProof); result.Outcome != Denied {
				t.Fatalf("result = %+v", result)
			}
			if !reflect.DeepEqual(changedPolicy, beforePolicy) || !reflect.DeepEqual(changedProof, beforeProof) {
				t.Fatal("authorization mutated policy or proof")
			}
		})
	}
}

func TestAuthorizeRejectsCurrentAuthorityAsRecoveryParticipant(t *testing.T) {
	t.Parallel()
	policy, signers := recoveryFixture(t, 2, 3)
	policy.Participants[0] = policy.CurrentAuthority
	sort.Slice(policy.Participants, func(i, j int) bool {
		return bytes.Compare(policy.Participants[i][:], policy.Participants[j][:]) < 0
	})
	proof := signedRecoveryProof(t, policy, signers[:2], Initiate)
	if result := Authorize(policy, proof); result.Outcome != Denied {
		t.Fatalf("result = %+v", result)
	}
}

func clonePolicy(policy Policy) Policy {
	policy.Participants = append([][32]byte(nil), policy.Participants...)
	return policy
}

func cloneProof(proof Proof) Proof {
	proof.Signatures = append([]Signature(nil), proof.Signatures...)
	for i := range proof.Signatures {
		proof.Signatures[i].Bytes = append([]byte(nil), proof.Signatures[i].Bytes...)
	}
	return proof
}

func recoveryFixture(t *testing.T, threshold, count int) (Policy, []ed25519.PrivateKey) {
	t.Helper()
	_, current, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	policy := Policy{Network: [32]byte{7}, Name: "alice", Generation: 3, Revision: 2,
		Threshold: uint8(threshold), Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], current.Public().(ed25519.PublicKey))
	var signers []ed25519.PrivateKey
	for i := 0; i < count; i++ {
		public, private, keyErr := ed25519.GenerateKey(nil)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		var participant [32]byte
		copy(participant[:], public)
		policy.Participants = append(policy.Participants, participant)
		signers = append(signers, private)
	}
	sort.Slice(policy.Participants, func(i, j int) bool {
		return bytes.Compare(policy.Participants[i][:], policy.Participants[j][:]) < 0
	})
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	return policy, signers
}

func signedRecoveryProof(t *testing.T, policy Policy, signers []ed25519.PrivateKey, operation Operation) Proof {
	t.Helper()
	proof := Proof{Operation: operation, PolicyDigest: PolicyDigest(policy),
		OperationID: sha256.Sum256([]byte("recovery-operation-1")), StartedAt: 1_000,
		CompletesAt: 1_000 + policy.Delay.Milliseconds()}
	proof.Successor = sha256.Sum256([]byte("successor-name-authority"))
	for _, private := range signers {
		public := private.Public().(ed25519.PublicKey)
		var signer [32]byte
		copy(signer[:], public)
		proof.Signatures = append(proof.Signatures, Signature{Signer: signer,
			Bytes: ed25519.Sign(private, RecoveryTranscript(policy, proof))})
	}
	sort.Slice(proof.Signatures, func(i, j int) bool {
		return bytes.Compare(proof.Signatures[i].Signer[:], proof.Signatures[j].Signer[:]) < 0
	})
	return proof
}
