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

func TestRecoveryPolicyDelayAndSuccessorFreshRecord(t *testing.T) {
	t.Parallel()
	policy, signers, currentPrivate, successor := lifecycleRecoveryFixture()
	currentAuthority := hex.EncodeToString(currentPrivate.Public().(ed25519.PublicKey))
	leasePolicy := namespace.Policy{DefaultLeaseDuration: 200 * time.Hour, DefaultGraceDuration: time.Hour}
	record, err := namespace.Apply(nil, 100, namespace.Op{Kind: "claim", Name: "alice",
		Generation: 1, Authority: currentAuthority}, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	activation := int64(101_000) + policy.Delay.Milliseconds()
	record, err = namespace.Apply(&record, 101, namespace.Op{Kind: "schedule-recovery-policy", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 1, Authority: currentAuthority,
		PolicyDigest: policy.Digest(), PolicyRevision: 1, PolicyDelay: policy.Delay,
		PolicyActivatesAt: activation}, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	proof := lifecycleProof(policy, signers, "initiate", activation+1_000, successor)
	authorization, err := policy.Authorize(proof)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := namespace.Apply(&record, activation/1_000-1, namespace.Op{Kind: "start-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		RecoveryAuthorization: authorization}, leasePolicy); err == nil {
		t.Fatal("pending Recovery Policy authorized recovery before activation")
	}
	record, err = namespace.Apply(&record, activation/1_000, namespace.Op{Kind: "activate-recovery-policy", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision}, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	record, err = namespace.Apply(&record, proof.StartedAt/1_000, namespace.Op{Kind: "start-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		RecoveryAuthorization: authorization}, leasePolicy)
	if err != nil || record.Recovery != "recovery-pending" {
		t.Fatalf("pending=%+v err=%v", record, err)
	}
	held, err := namespace.Apply(&record, proof.CompletesAt/1_000-1, namespace.Op{Kind: "advance", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision}, leasePolicy)
	if err != nil || held != record {
		t.Fatalf("Recovery Pending was not held exactly to its boundary: held=%+v err=%v", held, err)
	}
	automatic, err := namespace.Apply(&record, proof.CompletesAt/1_000, namespace.Op{Kind: "advance", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision}, leasePolicy)
	if err != nil || automatic.Authority != hex.EncodeToString(successor[:]) ||
		automatic.Consistency != "unavailable" || automatic.Recovery != "stable" {
		t.Fatalf("automatic recovery outcome=%+v err=%v", automatic, err)
	}
	if _, err := namespace.Apply(&record, proof.StartedAt/1_000+1, namespace.Op{Kind: "renew", Name: "alice",
		Authority: currentAuthority, ExpectedGeneration: 1, ExpectedRevision: record.Revision}, leasePolicy); err == nil {
		t.Fatal("current Authority renewed during Recovery Pending")
	}
	record, err = namespace.Apply(&record, proof.CompletesAt/1_000, namespace.Op{Kind: "complete-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		RecoveryAuthorization: authorization}, leasePolicy)
	if err != nil || record.Authority != hex.EncodeToString(successor[:]) || record.Recovery != "stable" ||
		record.Consistency != "unavailable" || record.Target != ([32]byte{}) {
		t.Fatalf("completed=%+v err=%v", record, err)
	}
	if _, err := namespace.Apply(&record, proof.CompletesAt/1_000+1, namespace.Op{Kind: "resume-recovery", Name: "alice",
		Authority: currentAuthority, ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		Target: [32]byte{9}}, leasePolicy); err == nil {
		t.Fatal("predecessor published the post-recovery Record")
	}
	record, err = namespace.Apply(&record, proof.CompletesAt/1_000+1, namespace.Op{Kind: "resume-recovery", Name: "alice",
		Authority: record.Authority, ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		Target: [32]byte{9}}, leasePolicy)
	if err != nil || record.Recovery != "stable" || record.Target != ([32]byte{9}) {
		t.Fatalf("resumed=%+v err=%v", record, err)
	}
}

func TestRecoveryCancellationRequiresDistinctThresholdDomain(t *testing.T) {
	t.Parallel()
	policy, signers, currentPrivate, successor := lifecycleRecoveryFixture()
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 3,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(currentPrivate.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000_000, GraceExpiresAt: 1_100_000,
		RecoveryPolicy: policy.Digest(), RecoveryPolicyRev: policy.Revision,
		RecoveryPolicyDelay: policy.Delay.Milliseconds(), Continuity: 1}
	started := int64(100_000)
	initiateProof := lifecycleProof(policy, signers, "initiate", started, successor)
	initiate, err := policy.Authorize(initiateProof)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := namespace.Apply(&record, started/1_000, namespace.Op{Kind: "start-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: 3, RecoveryAuthorization: initiate}, namespace.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := namespace.Apply(&pending, started/1_000+1, namespace.Op{Kind: "cancel-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: pending.Revision,
		RecoveryAuthorization: initiate}, namespace.Policy{}); err == nil {
		t.Fatal("initiation-domain signatures cancelled recovery")
	}
	cancelProof := lifecycleProof(policy, signers, "cancel", started, successor)
	cancel, err := policy.Authorize(cancelProof)
	if err != nil {
		t.Fatal(err)
	}
	stable, err := namespace.Apply(&pending, started/1_000+1, namespace.Op{Kind: "cancel-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: pending.Revision,
		RecoveryAuthorization: cancel}, namespace.Policy{})
	if err != nil || stable.Recovery != "stable" || stable.Authority != record.Authority {
		t.Fatalf("cancelled=%+v err=%v", stable, err)
	}
}

func TestPendingPolicyChangeKeepsPrecedingPolicyEffective(t *testing.T) {
	t.Parallel()
	oldPolicy, oldSigners, currentPrivate, successor := lifecycleRecoveryFixture()
	newPolicy := oldPolicy
	newPolicy.Revision++
	newPolicy.Delay = 96 * time.Hour
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 5,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(currentPrivate.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 10_000_000, GraceExpiresAt: 11_000_000,
		RecoveryPolicy: oldPolicy.Digest(), RecoveryPolicyRev: oldPolicy.Revision,
		RecoveryPolicyDelay: oldPolicy.Delay.Milliseconds(), Continuity: 1}
	activation := int64(200_000) + newPolicy.Delay.Milliseconds()
	scheduled, err := namespace.Apply(&record, 200, namespace.Op{Kind: "schedule-recovery-policy", Name: "alice",
		Authority: record.Authority, ExpectedGeneration: 1, ExpectedRevision: record.Revision,
		PolicyDigest: newPolicy.Digest(), PolicyRevision: newPolicy.Revision,
		PolicyDelay: newPolicy.Delay, PolicyActivatesAt: activation}, namespace.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	oldProof := lifecycleProof(oldPolicy, oldSigners, "initiate", 201_000, successor)
	oldAuthorization, err := oldPolicy.Authorize(oldProof)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := namespace.Apply(&scheduled, 201, namespace.Op{Kind: "start-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: scheduled.Revision,
		RecoveryAuthorization: oldAuthorization}, namespace.Policy{}); err != nil {
		t.Fatalf("preceding policy stopped before replacement activation: %v", err)
	}
	active, err := namespace.Apply(&scheduled, activation/1_000, namespace.Op{Kind: "activate-recovery-policy",
		Name: "alice", ExpectedGeneration: 1, ExpectedRevision: scheduled.Revision}, namespace.Policy{})
	if err != nil || active.RecoveryPolicy != newPolicy.Digest() {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	if _, err := namespace.Apply(&active, activation/1_000+1, namespace.Op{Kind: "start-recovery", Name: "alice",
		ExpectedGeneration: 1, ExpectedRevision: active.Revision,
		RecoveryAuthorization: oldAuthorization}, namespace.Policy{}); err == nil {
		t.Fatal("preceding policy authorized recovery after replacement")
	}
	disableAt := (activation/1_000+2)*1_000 + newPolicy.Delay.Milliseconds()
	disabling, err := namespace.Apply(&active, activation/1_000+2, namespace.Op{Kind: "schedule-recovery-policy",
		Name: "alice", Authority: active.Authority, ExpectedGeneration: 1, ExpectedRevision: active.Revision,
		PolicyRevision: newPolicy.Revision + 1, PolicyDelay: newPolicy.Delay,
		PolicyActivatesAt: disableAt}, namespace.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := namespace.Apply(&disabling, disableAt/1_000, namespace.Op{Kind: "activate-recovery-policy",
		Name: "alice", ExpectedGeneration: 1, ExpectedRevision: disabling.Revision}, namespace.Policy{})
	if err != nil || disabled.RecoveryPolicy != ([32]byte{}) || disabled.RecoveryPolicyRev != newPolicy.Revision+1 {
		t.Fatalf("disabled=%+v err=%v", disabled, err)
	}
}

func lifecycleRecoveryFixture() (namespace.RecoveryPolicy, []ed25519.PrivateKey, ed25519.PrivateKey, [32]byte) {
	currentSeed := sha256.Sum256([]byte("current"))
	current := ed25519.NewKeyFromSeed(currentSeed[:])
	policy := namespace.RecoveryPolicy{Network: [32]byte{7}, Name: "alice", Generation: 1,
		Revision: 1, Threshold: 2, Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], current.Public().(ed25519.PublicKey))
	var signers []ed25519.PrivateKey
	for index := range 3 {
		seed := sha256.Sum256([]byte{byte(index + 1)})
		private := ed25519.NewKeyFromSeed(seed[:])
		var participant [32]byte
		copy(participant[:], private.Public().(ed25519.PublicKey))
		policy.Participants = append(policy.Participants, participant)
		signers = append(signers, private)
	}
	sort.Slice(policy.Participants, func(i, j int) bool {
		return bytes.Compare(policy.Participants[i][:], policy.Participants[j][:]) < 0
	})
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	successorSeed := sha256.Sum256([]byte("successor"))
	successorPrivate := ed25519.NewKeyFromSeed(successorSeed[:])
	var successor [32]byte
	copy(successor[:], successorPrivate.Public().(ed25519.PublicKey))
	return policy, signers, current, successor
}

func lifecycleProof(policy namespace.RecoveryPolicy, signers []ed25519.PrivateKey,
	operation string, started int64, successor [32]byte,
) namespace.RecoveryProof {
	proof := namespace.RecoveryProof{Operation: operation, PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("operation")), Successor: successor,
		StartedAt: started, CompletesAt: started + policy.Delay.Milliseconds()}
	for _, private := range signers[:2] {
		var signer [32]byte
		copy(signer[:], private.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, namespace.Signature{Signer: signer,
			Bytes: ed25519.Sign(private, policy.Transcript(proof))})
	}
	return proof
}
