package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/recovery"
)

func TestAuthorityTransitionRequiresPredecessorAndPermanentlyInstallsSuccessor(t *testing.T) {
	t.Parallel()
	network := [32]byte{7}
	oldKey := deterministicAuthority("old")
	newKey := deterministicAuthority("new")
	current := authorityRecord(oldKey)
	op := record.Op{Kind: "rotate", Name: current.Name, Authority: current.Authority,
		SuccessorAuthority: hex.EncodeToString(newKey.Public().(ed25519.PublicKey)),
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision}
	proof, err := authority.SignTransition(network, current, op, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	gate, admissionProof := admittedTransition(t, network, current, op, [32]byte{1})
	rotated, err := authority.ApplyAdmittedTransition(gate, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, proof, 101, record.Policy{})
	if err != nil || rotated.Authority != op.SuccessorAuthority || rotated.Generation != current.Generation ||
		rotated.Target != current.Target {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
	replay := op
	replay.ExpectedRevision = rotated.Revision
	if _, err := authority.ApplyAdmittedTransition(gate, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, rotated, replay, proof, 102, record.Policy{}); err == nil {
		t.Fatal("predecessor transition proof replayed after successor installation")
	}
	if _, err := record.SignRecord(network, rotated, oldKey); err == nil {
		t.Fatal("predecessor signed a successor Record")
	}
	if _, err := record.SignRecord(network, rotated, newKey); err != nil {
		t.Fatalf("successor could not sign the new Record: %v", err)
	}
}

func TestAuthorityTransitionUsesOneSealedNamespaceSigningRequest(t *testing.T) {
	t.Parallel()
	network, key := [32]byte{17}, deterministicAuthority("sealed-transition")
	current := authorityRecord(key)
	op := record.Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: time.Hour}
	var requests int
	proof, err := authority.SignTransitionWith(network, current, op, transitionSignerFunc(func(request authority.TransitionSigningRequest) ([]byte, error) {
		requests++
		var expected [ed25519.PublicKeySize]byte
		copy(expected[:], key.Public().(ed25519.PublicKey))
		generation, revision := request.Predecessor()
		if request.Authority() != expected || generation != current.Generation || revision != current.Revision || len(request.Transcript()) == 0 {
			t.Fatal("Namespace supplied an invalid sealed transition request")
		}
		return ed25519.Sign(key, request.Transcript()), nil
	}))
	if err != nil || requests != 1 {
		t.Fatalf("sealed transition = %x, requests=%d, err=%v", proof, requests, err)
	}
	admission, admissionProof := admittedTransition(t, network, current, op, [32]byte{7})
	updated, err := authority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, proof, 101, record.Policy{})
	if err != nil || updated.Revision != current.Revision+1 {
		t.Fatalf("sealed transition did not apply: updated=%+v err=%v", updated, err)
	}
	if _, err := authority.SignTransitionWith(network, current, op, transitionSignerFunc(func(authority.TransitionSigningRequest) ([]byte, error) {
		return ed25519.Sign(key, []byte("substituted transcript")), nil
	})); err == nil {
		t.Fatal("sealed transition accepted a substituted transcript signature")
	}
}

type transitionSignerFunc func(authority.TransitionSigningRequest) ([]byte, error)

func (sign transitionSignerFunc) Sign(request authority.TransitionSigningRequest) ([]byte, error) {
	return sign(request)
}

func TestParentAuthorityDelegatesChosenChildAuthorityWithoutTransferringParent(t *testing.T) {
	t.Parallel()
	network := [32]byte{8}
	parentKey := deterministicAuthority("parent")
	childKey := deterministicAuthority("child")
	parent := authorityRecord(parentKey)
	parent.Name = "root"
	op := record.Op{Kind: "claim", Name: "leaf.root", Generation: 1,
		Authority: hex.EncodeToString(childKey.Public().(ed25519.PublicKey)), Parents: []record.Record{parent}}
	proof, err := authority.SignTransition(network, parent, op, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	admission, admissionProof := admittedTransition(t, network, parent, op, [32]byte{2})
	child, err := authority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, parent, op, proof, 101,
		record.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
	if err != nil || child.Authority != op.Authority || child.ParentName != parent.Name ||
		parent.Authority == child.Authority {
		t.Fatalf("child=%+v err=%v", child, err)
	}
}

func TestRecoveryTransitionRequiresThresholdAuthorizationAndAdmission(t *testing.T) {
	t.Parallel()
	network, currentKey := [32]byte{6}, deterministicAuthority("recovery-current")
	current := authorityRecord(currentKey)
	policy := recovery.RecoveryPolicy{Network: network, Name: current.Name, Generation: current.Generation,
		Revision: 1, Threshold: 2, Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], currentKey.Public().(ed25519.PublicKey))
	signers := []ed25519.PrivateKey{deterministicAuthority("recovery-1"), deterministicAuthority("recovery-2")}
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	for _, signer := range signers {
		var participant [32]byte
		copy(participant[:], signer.Public().(ed25519.PublicKey))
		policy.Participants = append(policy.Participants, participant)
	}
	current.RecoveryPolicy, current.RecoveryPolicyRev = policy.Digest(), policy.Revision
	current.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	proof := recovery.RecoveryProof{Operation: "initiate", PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("operation")), Successor: sha256.Sum256([]byte("successor")),
		StartedAt: 100_000, CompletesAt: 100_000 + policy.Delay.Milliseconds()}
	for _, signer := range signers {
		var id [32]byte
		copy(id[:], signer.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, recovery.Signature{Signer: id,
			Bytes: ed25519.Sign(signer, policy.Transcript(proof))})
	}
	authorization, err := policy.Authorize(proof)
	if err != nil {
		t.Fatal(err)
	}
	op := record.Op{Kind: "start-recovery", Name: current.Name, ExpectedGeneration: current.Generation,
		ExpectedRevision: current.Revision, RecoveryAuthorization: authorization}
	gate, admissionProof := admittedTransition(t, network, current, op, [32]byte{3})
	if _, err := authority.ApplyAdmittedTransition(gate, admission.Proof{}, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, nil, 100, record.Policy{}); err == nil {
		t.Fatal("threshold authorization bypassed anonymous admission")
	}
	pending, err := authority.ApplyAdmittedTransition(gate, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, nil, 100, record.Policy{})
	if err != nil || pending.Recovery != "recovery-pending" {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}

func admittedTransition(t *testing.T, network [32]byte, current record.Record,
	op record.Op, isolation [32]byte,
) (*admission.Admission, admission.Proof) {
	t.Helper()
	digest, err := authority.TransitionDigest(network, current, op)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := admission.NewAdmission([32]byte{9}, network, 1, [32]byte{8})
	if err != nil {
		t.Fatal(err)
	}
	surface := "renewal-update"
	if op.Kind == "start-recovery" || op.Kind == "cancel-recovery" || op.Kind == "complete-recovery" ||
		op.Kind == "schedule-recovery-policy" || op.Kind == "resume-recovery" {
		surface = "policy-recovery"
	}
	challenge, err := gate.Issue(100_000, surface, digest, isolation, 110_000, [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	return gate, proof
}

func deterministicAuthority(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func authorityRecord(private ed25519.PrivateKey) record.Record {
	return record.Record{Name: "alice", Generation: 1, Revision: 2,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
}
