package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestRecoveryPolicyAuthorizeRequiresDistinctThresholdProof(t *testing.T) {
	t.Parallel()
	policy, signers := recoveryFixture(t, 2, 3)
	proof := signedProof(policy, signers[:2], "initiate")
	authorization, err := policy.Authorize(proof)
	if err != nil || authorization.ValidSigners != 2 || authorization.Operation != "initiate" ||
		!authorization.Verified() {
		t.Fatalf("authorization=%+v err=%v", authorization, err)
	}
	authorization.Successor[0]++
	if authorization.Verified() {
		t.Fatal("field-mutated authorization retained verifier provenance")
	}
}

func TestRecoveryPolicyAuthorizeRejectsHostileProofs(t *testing.T) {
	policy, signers := recoveryFixture(t, 2, 3)
	valid := signedProof(policy, signers[:2], "initiate")
	tests := map[string]func(*namespace.RecoveryPolicy, *namespace.RecoveryProof){
		"threshold minus one": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Signatures = proof.Signatures[:1]
		},
		"duplicate signer": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Signatures[1] = proof.Signatures[0]
		},
		"unknown signer": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Signatures[0].Signer = [32]byte{99}
		},
		"wrong generation": func(policy *namespace.RecoveryPolicy, _ *namespace.RecoveryProof) {
			policy.Generation++
		},
		"wrong policy": func(policy *namespace.RecoveryPolicy, _ *namespace.RecoveryProof) {
			policy.Revision++
		},
		"changed successor": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Successor[0]++
		},
		"changed boundary": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.CompletesAt++
		},
		"wrong domain": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Operation = "cancel"
		},
		"malformed signature": func(_ *namespace.RecoveryPolicy, proof *namespace.RecoveryProof) {
			proof.Signatures[0].Bytes = []byte{1}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changedPolicy := clonePolicy(policy)
			changedProof := cloneProof(valid)
			mutate(&changedPolicy, &changedProof)
			if _, err := changedPolicy.Authorize(changedProof); err == nil {
				t.Fatal("hostile proof authorized")
			}
		})
	}
}

func TestRecoveryPolicyDigestRejectsInvalidParticipantSets(t *testing.T) {
	policy, _ := recoveryFixture(t, 2, 3)
	for name, mutate := range map[string]func(*namespace.RecoveryPolicy){
		"threshold":         func(value *namespace.RecoveryPolicy) { value.Threshold = 1 },
		"duplicate":         func(value *namespace.RecoveryPolicy) { value.Participants[1] = value.Participants[0] },
		"current authority": func(value *namespace.RecoveryPolicy) { value.Participants[0] = value.CurrentAuthority },
	} {
		t.Run(name, func(t *testing.T) {
			changed := clonePolicy(policy)
			mutate(&changed)
			if changed.Digest() != [32]byte{} {
				t.Fatal("invalid Recovery Policy received a commitment")
			}
		})
	}
}

func recoveryFixture(t *testing.T, threshold, count int) (namespace.RecoveryPolicy, []ed25519.PrivateKey) {
	t.Helper()
	current, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy := namespace.RecoveryPolicy{Network: [32]byte{7}, Name: "alice", Generation: 3,
		Revision: 2, Threshold: uint8(threshold), Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], current)
	var signers []ed25519.PrivateKey
	for range count {
		public, private, keyErr := ed25519.GenerateKey(rand.Reader)
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

func signedProof(policy namespace.RecoveryPolicy, signers []ed25519.PrivateKey, operation string) namespace.RecoveryProof {
	proof := namespace.RecoveryProof{Operation: operation, PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("recovery-operation-1")), Successor: sha256.Sum256([]byte("successor")),
		StartedAt: 1_000, CompletesAt: 1_000 + policy.Delay.Milliseconds()}
	for _, private := range signers {
		var signer [32]byte
		copy(signer[:], private.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, namespace.Signature{Signer: signer,
			Bytes: ed25519.Sign(private, policy.Transcript(proof))})
	}
	return proof
}

func clonePolicy(policy namespace.RecoveryPolicy) namespace.RecoveryPolicy {
	policy.Participants = append([][32]byte(nil), policy.Participants...)
	return policy
}

func cloneProof(proof namespace.RecoveryProof) namespace.RecoveryProof {
	proof.Signatures = append([]namespace.Signature(nil), proof.Signatures...)
	for index := range proof.Signatures {
		proof.Signatures[index].Bytes = append([]byte(nil), proof.Signatures[index].Bytes...)
	}
	return proof
}
