package nameauthority_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/namerecovery"
)

func TestAuthorityTransitionRequiresPredecessorAndPermanentlyInstallsSuccessor(t *testing.T) {
	t.Parallel()
	network := [32]byte{7}
	oldKey := deterministicAuthority("old")
	newKey := deterministicAuthority("new")
	record := authorityRecord(oldKey)
	op := namelease.Op{Kind: "rotate", Name: record.Name, Authority: record.Authority,
		SuccessorAuthority: hex.EncodeToString(newKey.Public().(ed25519.PublicKey)),
		ExpectedGeneration: record.Generation, ExpectedRevision: record.Revision}
	proof, err := nameauthority.SignTransition(network, record, op, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	admission, admissionProof := admittedTransition(t, network, record, op, [32]byte{1})
	rotated, err := nameauthority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, proof, 101, namelease.Policy{})
	if err != nil || rotated.Authority != op.SuccessorAuthority || rotated.Generation != record.Generation ||
		rotated.Target != record.Target {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
	replay := op
	replay.ExpectedRevision = rotated.Revision
	if _, err := nameauthority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, rotated, replay, proof, 102, namelease.Policy{}); err == nil {
		t.Fatal("predecessor transition proof replayed after successor installation")
	}
	if _, err := nameauthority.SignRecord(network, rotated, oldKey); err == nil {
		t.Fatal("predecessor signed a successor Record")
	}
	if _, err := nameauthority.SignRecord(network, rotated, newKey); err != nil {
		t.Fatalf("successor could not sign the new Record: %v", err)
	}
}

func TestParentAuthorityDelegatesChosenChildAuthorityWithoutTransferringParent(t *testing.T) {
	t.Parallel()
	network := [32]byte{8}
	parentKey := deterministicAuthority("parent")
	childKey := deterministicAuthority("child")
	parent := authorityRecord(parentKey)
	parent.Name = "root"
	op := namelease.Op{Kind: "claim", Name: "leaf.root", Generation: 1,
		Authority: hex.EncodeToString(childKey.Public().(ed25519.PublicKey)), Parents: []namelease.Record{parent}}
	proof, err := nameauthority.SignTransition(network, parent, op, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	admission, admissionProof := admittedTransition(t, network, parent, op, [32]byte{2})
	child, err := nameauthority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, parent, op, proof, 101,
		namelease.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
	if err != nil || child.Authority != op.Authority || child.ParentName != parent.Name ||
		parent.Authority == child.Authority {
		t.Fatalf("child=%+v err=%v", child, err)
	}
}

func TestRecoveryTransitionRequiresThresholdAuthorizationAndAdmission(t *testing.T) {
	t.Parallel()
	network, currentKey := [32]byte{6}, deterministicAuthority("recovery-current")
	record := authorityRecord(currentKey)
	policy := namerecovery.RecoveryPolicy{Network: network, Name: record.Name, Generation: record.Generation,
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
	proof := namerecovery.Proof{Operation: "initiate", PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("operation")), Successor: sha256.Sum256([]byte("successor")),
		StartedAt: 100_000, CompletesAt: 100_000 + policy.Delay.Milliseconds()}
	for _, signer := range signers {
		var id [32]byte
		copy(id[:], signer.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, namerecovery.Signature{Signer: id,
			Bytes: ed25519.Sign(signer, policy.Transcript(proof))})
	}
	authorization, err := policy.Authorize(proof)
	if err != nil {
		t.Fatal(err)
	}
	op := namelease.Op{Kind: "start-recovery", Name: record.Name, ExpectedGeneration: record.Generation,
		ExpectedRevision: record.Revision, RecoveryAuthorization: authorization}
	admission, admissionProof := admittedTransition(t, network, record, op, [32]byte{3})
	if _, err := nameauthority.ApplyAdmittedTransition(admission, nameadmission.Proof{}, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, nil, 100, namelease.Policy{}); err == nil {
		t.Fatal("threshold authorization bypassed anonymous admission")
	}
	pending, err := nameauthority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, record, op, nil, 100, namelease.Policy{})
	if err != nil || pending.Recovery != "recovery-pending" {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}

func admittedTransition(t *testing.T, network [32]byte, current namelease.Record,
	op namelease.Op, isolation [32]byte,
) (*nameadmission.Admission, nameadmission.Proof) {
	t.Helper()
	digest, err := nameauthority.TransitionDigest(network, current, op)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := nameadmission.NewAdmission([32]byte{9}, network, 1, [32]byte{8})
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

func authorityRecord(private ed25519.PrivateKey) namelease.Record {
	return namelease.Record{Name: "alice", Generation: 1, Revision: 2,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
}
