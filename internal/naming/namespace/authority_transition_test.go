package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestAuthorityTransitionRequiresPredecessorAndPermanentlyInstallsSuccessor(t *testing.T) {
	t.Parallel()
	network := [32]byte{7}
	oldKey := deterministicAuthority("old")
	newKey := deterministicAuthority("new")
	record := authorityRecord(oldKey)
	op := namespace.Op{Kind: "rotate", Name: record.Name, Authority: record.Authority,
		SuccessorAuthority: hex.EncodeToString(newKey.Public().(ed25519.PublicKey)),
		ExpectedGeneration: record.Generation, ExpectedRevision: record.Revision}
	proof, err := namespace.SignTransition(network, record, op, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	admission, admissionProof := admittedTransition(t, network, record, op, [32]byte{1})
	rotated, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, proof, 101, namespace.Policy{})
	if err != nil || rotated.Authority != op.SuccessorAuthority || rotated.Generation != record.Generation ||
		rotated.Target != record.Target {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
	replay := op
	replay.ExpectedRevision = rotated.Revision
	if _, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, rotated, replay, proof, 102, namespace.Policy{}); err == nil {
		t.Fatal("predecessor transition proof replayed after successor installation")
	}
	if _, err := namespace.SignRecord(network, rotated, oldKey); err == nil {
		t.Fatal("predecessor signed a successor Record")
	}
	if _, err := namespace.SignRecord(network, rotated, newKey); err != nil {
		t.Fatalf("successor could not sign the new Record: %v", err)
	}
}

func TestAuthorityTransitionUsesOneSealedNamespaceSigningRequest(t *testing.T) {
	t.Parallel()
	network, key := [32]byte{17}, deterministicAuthority("sealed-transition")
	current := authorityRecord(key)
	op := namespace.Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: time.Hour}
	var requests int
	proof, err := namespace.SignTransitionWith(network, current, op, transitionSignerFunc(func(request namespace.TransitionSigningRequest) ([]byte, error) {
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
	updated, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, proof, 101, namespace.Policy{})
	if err != nil || updated.Revision != current.Revision+1 {
		t.Fatalf("sealed transition did not apply: updated=%+v err=%v", updated, err)
	}
	if _, err := namespace.SignTransitionWith(network, current, op, transitionSignerFunc(func(namespace.TransitionSigningRequest) ([]byte, error) {
		return ed25519.Sign(key, []byte("substituted transcript")), nil
	})); err == nil {
		t.Fatal("sealed transition accepted a substituted transcript signature")
	}
}

type transitionSignerFunc func(namespace.TransitionSigningRequest) ([]byte, error)

func (sign transitionSignerFunc) Sign(request namespace.TransitionSigningRequest) ([]byte, error) {
	return sign(request)
}

func TestParentAuthorityDelegatesChosenChildAuthorityWithoutTransferringParent(t *testing.T) {
	t.Parallel()
	network := [32]byte{8}
	parentKey := deterministicAuthority("parent")
	childKey := deterministicAuthority("child")
	parent := authorityRecord(parentKey)
	parent.Name = "root"
	op := namespace.Op{Kind: "claim", Name: "leaf.root", Generation: 1,
		Authority: hex.EncodeToString(childKey.Public().(ed25519.PublicKey)), Parents: []namespace.Record{parent}}
	proof, err := namespace.SignTransition(network, parent, op, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	admission, admissionProof := admittedTransition(t, network, parent, op, [32]byte{2})
	child, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, parent, op, proof, 101,
		namespace.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
	if err != nil || child.Authority != op.Authority || child.ParentName != parent.Name ||
		parent.Authority == child.Authority {
		t.Fatalf("child=%+v err=%v", child, err)
	}
}

func TestRecoveryTransitionRequiresThresholdAuthorizationAndAdmission(t *testing.T) {
	t.Parallel()
	network, currentKey := [32]byte{6}, deterministicAuthority("recovery-current")
	record := authorityRecord(currentKey)
	policy := namespace.RecoveryPolicy{Network: network, Name: record.Name, Generation: record.Generation,
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
	record.RecoveryPolicy, record.RecoveryPolicyRev = policy.Digest(), policy.Revision
	record.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	proof := namespace.RecoveryProof{Operation: "initiate", PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("operation")), Successor: sha256.Sum256([]byte("successor")),
		StartedAt: 100_000, CompletesAt: 100_000 + policy.Delay.Milliseconds()}
	for _, signer := range signers {
		var id [32]byte
		copy(id[:], signer.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, namespace.Signature{Signer: id,
			Bytes: ed25519.Sign(signer, policy.Transcript(proof))})
	}
	authorization, err := policy.Authorize(proof)
	if err != nil {
		t.Fatal(err)
	}
	op := namespace.Op{Kind: "start-recovery", Name: record.Name, ExpectedGeneration: record.Generation,
		ExpectedRevision: record.Revision, RecoveryAuthorization: authorization}
	admission, admissionProof := admittedTransition(t, network, record, op, [32]byte{3})
	if _, err := namespace.ApplyAdmittedTransition(admission, namespace.Proof{}, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, nil, 100, namespace.Policy{}); err == nil {
		t.Fatal("threshold authorization bypassed anonymous admission")
	}
	pending, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, nil, 100, namespace.Policy{})
	if err != nil || pending.Recovery != "recovery-pending" {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}

func admittedTransition(t *testing.T, network [32]byte, current namespace.Record,
	op namespace.Op, isolation [32]byte,
) (*namespace.Admission, namespace.Proof) {
	t.Helper()
	digest, err := namespace.TransitionDigest(network, current, op)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := namespace.NewAdmission([32]byte{9}, network, 1, [32]byte{8})
	if err != nil {
		t.Fatal(err)
	}
	surface := "renewal-update"
	if op.Kind == "start-recovery" || op.Kind == "cancel-recovery" || op.Kind == "complete-recovery" ||
		op.Kind == "schedule-recovery-policy" || op.Kind == "resume-recovery" {
		surface = "policy-recovery"
	}
	challenge, err := admission.Issue(100_000, surface, digest, isolation, 110_000, [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	return admission, proof
}

func deterministicAuthority(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func authorityRecord(private ed25519.PrivateKey) namespace.Record {
	return namespace.Record{Name: "alice", Generation: 1, Revision: 2,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
}
