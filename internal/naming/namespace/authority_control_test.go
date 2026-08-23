package namespace

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"testing"
	"time"
)

func TestControlAppliesTheAdmittedCanonicalOperation(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 123_000_000).UTC()
	network, node, isolation := [32]byte{9}, [32]byte{2}, [32]byte{3}
	seed := sha256.Sum256([]byte("control-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	current := Record{Name: "alice", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable",
		Authority:      hex.EncodeToString(private.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(), Continuity: 1}
	policy := Policy{DefaultLeaseDuration: 2 * time.Hour, DefaultGraceDuration: time.Hour}
	op := Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision,
		LeaseDuration: policy.DefaultLeaseDuration}
	signature, err := SignTransition(network, current, op, private)
	if err != nil {
		t.Fatal(err)
	}
	operation := controlOperation{Kind: "renew", Name: current.Name, Generation: current.Generation,
		ExpectedRevision: current.Revision, LeaseNotAfter: now.Add(policy.DefaultLeaseDuration).UnixMilli(),
		AuthorityProof: signature}
	raw, digest := signedControlOperation(t, operation)
	gate, err := NewAdmission(node, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := gate.Issue(now.UnixMilli(), "renewal-update", digest, isolation,
		now.Add(15*time.Second).UnixMilli(), [16]byte{5})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	control, err := NewEvidenceControl(network, gate, ClaimOrder{}, []Record{current},
		func() time.Time { return now }, policy)
	if err != nil {
		t.Fatal(err)
	}
	class, generation, revision, state := control.ApplyEvidence(raw, proof)
	updated, decodeErr := DecodeRecord(state)
	if class != "accepted" || generation != 1 || revision != 2 || decodeErr != nil ||
		updated.LeaseExpiresAt != now.Add(policy.DefaultLeaseDuration).Unix() {
		t.Fatalf("class=%q generation=%d revision=%d updated=%+v decode=%v", class, generation, revision, updated, decodeErr)
	}
}

func TestControlRejectsChangedContentsWithTheOldAdmission(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	gate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	operation := controlOperation{Kind: "renew", Name: "alice", Generation: 1, ExpectedRevision: 1,
		LeaseNotAfter: now.Add(time.Hour).UnixMilli(), AuthorityProof: []byte{1}}
	_, digest := signedControlOperation(t, operation)
	operation.LeaseNotAfter++
	raw, _ := signedControlOperation(t, operation)
	challenge, err := gate.Issue(now.UnixMilli(), "renewal-update", digest, [32]byte{3},
		now.Add(15*time.Second).UnixMilli(), [16]byte{5})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	control, err := NewEvidenceControl(network, gate, ClaimOrder{}, nil, func() time.Time { return now }, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if class, _, _, _ := control.ApplyEvidence(raw, proof); class != "denied" {
		t.Fatal("changed control contents reused the old admission")
	}
}

func TestControlOwnsMultilevelParentLineage(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	rootKey := deterministicControlKey("lineage-root")
	childKey := deterministicControlKey("lineage-child")
	grandchildKey := deterministicControlKey("lineage-grandchild")
	root := controlTestRecord("root", rootKey, now)
	gate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewEvidenceControl(network, gate, ClaimOrder{}, []Record{root},
		func() time.Time { return now }, Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}

	child := controlDelegate(t, network, root, rootKey, "child.root", childKey, now)
	class, _, _, state := applyControlTest(t, control, gate, now, network, child, 21)
	childRecord, err := DecodeRecord(state)
	if class != "accepted" || err != nil || childRecord.ParentName != root.Name {
		t.Fatalf("child delegation = %q, %+v, %v", class, childRecord, err)
	}

	grandchild := controlDelegate(t, network, childRecord, childKey, "grandchild.child.root", grandchildKey, now)
	class, _, _, state = applyControlTest(t, control, gate, now, network, grandchild, 22)
	grandchildRecord, err := DecodeRecord(state)
	if class != "accepted" || err != nil || grandchildRecord.ParentName != childRecord.Name {
		t.Fatalf("grandchild delegation = %q, %+v, %v", class, grandchildRecord, err)
	}

	publishOp := Op{Kind: "publish", Name: grandchildRecord.Name, Authority: grandchildRecord.Authority,
		ExpectedGeneration: grandchildRecord.Generation, ExpectedRevision: grandchildRecord.Revision,
		Target: [32]byte{7}, RecordNotAfter: now.Add(30 * time.Minute).UnixMilli()}
	publishProof, err := SignTransition(network, grandchildRecord, publishOp, grandchildKey)
	if err != nil {
		t.Fatal(err)
	}
	publish := controlOperation{Kind: "record", Name: grandchildRecord.Name, Generation: grandchildRecord.Generation,
		ExpectedRevision: grandchildRecord.Revision, Target: [32]byte{7},
		RecordNotAfter: now.Add(30 * time.Minute).UnixMilli(), AuthorityProof: publishProof}
	class, _, _, state = applyControlTest(t, control, gate, now, network, publish, 23)
	published, err := DecodeRecord(state)
	if class != "accepted" || err != nil || published.Target != ([32]byte{7}) {
		t.Fatalf("grandchild publication = %q, %+v, %v", class, published, err)
	}

	renewOp := Op{Kind: "renew", Name: published.Name, Authority: published.Authority,
		ExpectedGeneration: published.Generation, ExpectedRevision: published.Revision, LeaseDuration: time.Hour}
	renewProof, err := SignTransition(network, published, renewOp, grandchildKey)
	if err != nil {
		t.Fatal(err)
	}
	renew := controlOperation{Kind: "renew", Name: published.Name, Generation: published.Generation,
		ExpectedRevision: published.Revision, LeaseNotAfter: now.Add(time.Hour).UnixMilli(), AuthorityProof: renewProof}
	class, _, _, state = applyControlTest(t, control, gate, now, network, renew, 24)
	renewed, err := DecodeRecord(state)
	if class != "accepted" || err != nil || renewed.Revision != published.Revision+1 {
		t.Fatalf("grandchild renewal = %q, %+v, %v", class, renewed, err)
	}

	releaseOp := Op{Kind: "release", Name: root.Name, Authority: root.Authority,
		ExpectedGeneration: root.Generation, ExpectedRevision: root.Revision}
	releaseProof, err := SignTransition(network, root, releaseOp, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	release := controlOperation{Kind: "release", Name: root.Name, Generation: root.Generation,
		ExpectedRevision: root.Revision, AuthorityProof: releaseProof}
	if class, _, _, _ = applyControlTest(t, control, gate, now, network, release, 25); class != "accepted" {
		t.Fatal("root release was denied")
	}
	renewedOp := Op{Kind: "renew", Name: renewed.Name, Authority: renewed.Authority,
		ExpectedGeneration: renewed.Generation, ExpectedRevision: renewed.Revision, LeaseDuration: time.Hour}
	renewedProof, err := SignTransition(network, renewed, renewedOp, grandchildKey)
	if err != nil {
		t.Fatal(err)
	}
	renew.ExpectedRevision, renew.AuthorityProof = renewed.Revision, renewedProof
	if class, _, _, _ = applyControlTest(t, control, gate, now, network, renew, 26); class != "denied" {
		t.Fatal("released root allowed its descendant renewal")
	}
}

func controlDelegate(t *testing.T, network [32]byte, parent Record, parentKey ed25519.PrivateKey,
	name string, childKey ed25519.PrivateKey, now time.Time,
) controlOperation {
	t.Helper()
	childAuthority := authorityBytes(hex.EncodeToString(childKey.Public().(ed25519.PublicKey)))
	op := Op{Kind: "claim", Name: name, Generation: 1, Authority: hex.EncodeToString(childAuthority[:]),
		Parents: []Record{parent}, LeaseDuration: time.Hour}
	proof, err := SignTransition(network, parent, op, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	return controlOperation{Kind: "delegate", Name: name, ParentName: parent.Name,
		ParentGeneration: parent.Generation, ParentRevision: parent.Revision, ChildGeneration: 1,
		Authority: childAuthority, LeaseNotAfter: now.Add(time.Hour).UnixMilli(), AuthorityProof: proof}
}

func TestControlEnforcesPolicyDelayAndSupportsDisable(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network, currentKey := [32]byte{9}, deterministicControlKey("policy-current")
	current := controlTestRecord("policy-name", currentKey, now)
	policy, _ := controlTestRecoveryPolicy(network, current, currentKey, 30*24*time.Hour)
	policyRaw, _ := json.Marshal(policy)
	leasePolicy := Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	gate, _ := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	clock := now
	control, err := NewEvidenceControl(network, gate, ClaimOrder{}, []Record{current},
		func() time.Time { return clock }, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	early := controlOperation{Kind: "policy", Name: current.Name, Generation: 1, ExpectedRevision: 1,
		PolicyNotBefore: now.Add(72 * time.Hour).UnixMilli(), RecoveryPolicy: policyRaw}
	earlyOp := Op{Kind: "schedule-recovery-policy", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: 1, ExpectedRevision: 1, PolicyDigest: policy.Digest(), PolicyRevision: 1,
		PolicyDelay: 72 * time.Hour, PolicyActivatesAt: early.PolicyNotBefore}
	early.AuthorityProof, _ = SignTransition(network, current, earlyOp, currentKey)
	if class, _, _, _ := applyControlTest(t, control, gate, clock, network, early, 1); class != "denied" {
		t.Fatal("policy was scheduled earlier than its own delay")
	}
	valid := early
	valid.PolicyNotBefore = now.Add(policy.Delay).UnixMilli()
	validOp := earlyOp
	validOp.PolicyDelay, validOp.PolicyActivatesAt = policy.Delay, valid.PolicyNotBefore
	valid.AuthorityProof, _ = SignTransition(network, current, validOp, currentKey)
	class, _, _, state := applyControlTest(t, control, gate, clock, network, valid, 2)
	updated, decodeErr := DecodeRecord(state)
	if class != "accepted" || decodeErr != nil || updated.PendingPolicy != policy.Digest() {
		t.Fatalf("policy class=%q state=%+v err=%v", class, updated, decodeErr)
	}

	effective := current
	effective.RecoveryPolicy, effective.RecoveryPolicyRev = policy.Digest(), 1
	effective.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	disableControl, _ := NewEvidenceControl(network, gate, ClaimOrder{}, []Record{effective},
		func() time.Time { return clock }, leasePolicy)
	disableOp := Op{Kind: "schedule-recovery-policy", Name: effective.Name, Authority: effective.Authority,
		ExpectedGeneration: 1, ExpectedRevision: 1, PolicyRevision: 2, PolicyDelay: policy.Delay,
		PolicyActivatesAt: now.Add(policy.Delay).UnixMilli()}
	invalidPolicy := policy
	invalidPolicy.Name, invalidPolicy.Threshold = effective.Name, 0
	invalidRaw, _ := json.Marshal(invalidPolicy)
	invalid := controlOperation{Kind: "policy", Name: effective.Name, Generation: 1, ExpectedRevision: 1,
		PolicyNotBefore: disableOp.PolicyActivatesAt, RecoveryPolicy: invalidRaw}
	invalid.AuthorityProof, _ = SignTransition(network, effective, disableOp, currentKey)
	if class, _, _, _ := applyControlTest(t, disableControl, gate, clock, network, invalid, 3); class != "denied" {
		t.Fatal("invalid replacement policy was interpreted as disable")
	}
	disable := controlOperation{Kind: "policy", Name: effective.Name, Generation: 1, ExpectedRevision: 1,
		PolicyNotBefore: disableOp.PolicyActivatesAt}
	disable.AuthorityProof, _ = SignTransition(network, effective, disableOp, currentKey)
	class, _, _, state = applyControlTest(t, disableControl, gate, clock, network, disable, 9)
	updated, decodeErr = DecodeRecord(state)
	if class != "accepted" || decodeErr != nil || updated.PendingPolicy != [32]byte{} || updated.PendingPolicyRev != 2 {
		t.Fatalf("disable class=%q state=%+v err=%v", class, updated, decodeErr)
	}
}

func TestControlExecutesRecoveryCancelCompleteAndResume(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network, currentKey := [32]byte{9}, deterministicControlKey("recovery-current")
	current := controlTestRecord("recovery-name", currentKey, now)
	policy, signers := controlTestRecoveryPolicy(network, current, currentKey, 72*time.Hour)
	current.RecoveryPolicy, current.RecoveryPolicyRev = policy.Digest(), policy.Revision
	current.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	gate, _ := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	clock := now
	control, _ := NewEvidenceControl(network, gate, ClaimOrder{}, []Record{current},
		func() time.Time { return clock }, Policy{})
	successor := deterministicControlKey("recovery-successor")
	initiate := controlTestRecoveryOperation(t, policy, signers, "initiate", [32]byte{7}, successor, clock)
	class, _, _, state := applyControlTest(t, control, gate, clock, network, initiate, 4)
	pending, err := DecodeRecord(state)
	if class != "accepted" || err != nil || pending.Recovery != "recovery-pending" {
		t.Fatalf("initiate class=%q state=%+v err=%v", class, pending, err)
	}
	clock = clock.Add(time.Second)
	cancel := controlTestRecoveryOperation(t, policy, signers, "cancel", [32]byte{7}, successor, now)
	cancel.ExpectedRevision, cancel.RecoveryNotBefore = pending.Revision, clock.UnixMilli()
	class, _, _, state = applyControlTest(t, control, gate, clock, network, cancel, 5)
	stable, err := DecodeRecord(state)
	if class != "accepted" || err != nil || stable.Recovery != "stable" {
		t.Fatalf("cancel class=%q state=%+v err=%v", class, stable, err)
	}
	clock = clock.Add(time.Second)
	initiate = controlTestRecoveryOperation(t, policy, signers, "initiate", [32]byte{8}, successor, clock)
	initiate.ExpectedRevision = stable.Revision
	_, _, _, state = applyControlTest(t, control, gate, clock, network, initiate, 6)
	pending, _ = DecodeRecord(state)
	clock = time.UnixMilli(pending.RecoveryExpiresAt)
	complete := initiate
	complete.ExpectedRevision, complete.RecoveryStep = pending.Revision, "complete"
	complete.RecoveryNotBefore = pending.RecoveryExpiresAt
	class, _, _, state = applyControlTest(t, control, gate, clock, network, complete, 7)
	completed, err := DecodeRecord(state)
	if class != "accepted" || err != nil || completed.Consistency != "unavailable" {
		t.Fatalf("complete class=%q state=%+v err=%v", class, completed, err)
	}
	clock = clock.Add(time.Second)
	resumeOp := Op{Kind: "resume-recovery", Name: completed.Name, Authority: completed.Authority,
		ExpectedGeneration: 1, ExpectedRevision: completed.Revision, Target: [32]byte{9},
		RecordNotAfter: clock.Add(time.Hour).UnixMilli()}
	resumeProof, _ := SignTransition(network, completed, resumeOp, successor)
	resume := controlOperation{Kind: "recovery", Name: completed.Name, Generation: 1,
		ExpectedRevision: completed.Revision, PolicyID: policy.Digest(), RecoveryStep: "resume",
		RecoveryNotBefore: clock.UnixMilli(), Target: [32]byte{9}, RecordNotAfter: resumeOp.RecordNotAfter,
		AuthorityProof: resumeProof}
	class, _, _, state = applyControlTest(t, control, gate, clock, network, resume, 8)
	resumed, err := DecodeRecord(state)
	if class != "accepted" || err != nil || resumed.Target != ([32]byte{9}) || resumed.Consistency != "current" {
		t.Fatalf("resume class=%q state=%+v err=%v", class, resumed, err)
	}
}

func signedControlOperation(t *testing.T, operation controlOperation) ([]byte, [32]byte) {
	t.Helper()
	operation.OperationDigest = [32]byte{}
	canonical, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	digest := controlOperationDigest(canonical)
	operation.OperationDigest = digest
	raw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	return raw, digest
}

func applyControlTest(t *testing.T, control *control, gate *Admission, now time.Time,
	network [32]byte, operation controlOperation, salt byte,
) (string, uint64, uint64, []byte) {
	t.Helper()
	raw, digest := signedControlOperation(t, operation)
	challenge, err := gate.Issue(now.UnixMilli(), operation.surface(), digest, [32]byte{salt},
		now.Add(15*time.Second).UnixMilli(), [16]byte{salt})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	return control.ApplyEvidence(raw, proof)
}

func deterministicControlKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func controlTestRecord(name string, key ed25519.PrivateKey, now time.Time) Record {
	return Record{Name: name, Generation: 1, Revision: 1, Lease: "active", Consistency: "current",
		Recovery: "stable", Authority: hex.EncodeToString(key.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: now.Add(200 * 24 * time.Hour).Unix(), GraceExpiresAt: now.Add(201 * 24 * time.Hour).Unix(),
		Continuity: 1}
}

func controlTestRecoveryPolicy(network [32]byte, record Record, current ed25519.PrivateKey,
	delay time.Duration,
) (RecoveryPolicy, []ed25519.PrivateKey) {
	policy := RecoveryPolicy{Network: network, Name: record.Name, Generation: record.Generation,
		Revision: 1, CurrentAuthority: authorityBytes(record.Authority), Threshold: 2, Delay: delay}
	signers := []ed25519.PrivateKey{deterministicControlKey("recovery-1"), deterministicControlKey("recovery-2")}
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	for _, signer := range signers {
		var participant [32]byte
		copy(participant[:], signer.Public().(ed25519.PublicKey))
		policy.Participants = append(policy.Participants, participant)
	}
	return policy, signers
}

func controlTestRecoveryOperation(t *testing.T, policy RecoveryPolicy,
	signers []ed25519.PrivateKey, step string, operationID [32]byte, successor ed25519.PrivateKey,
	started time.Time,
) controlOperation {
	t.Helper()
	proof := RecoveryProof{Operation: step, PolicyDigest: policy.Digest(), OperationID: operationID,
		Successor: authorityBytes(hex.EncodeToString(successor.Public().(ed25519.PublicKey))),
		StartedAt: started.UnixMilli(), CompletesAt: started.Add(policy.Delay).UnixMilli()}
	for _, signer := range signers {
		var identity [32]byte
		copy(identity[:], signer.Public().(ed25519.PublicKey))
		proof.Signatures = append(proof.Signatures, Signature{Signer: identity,
			Bytes: ed25519.Sign(signer, policy.Transcript(proof))})
	}
	raw, err := json.Marshal(recoveryEnvelope{Policy: policy, Proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	return controlOperation{Kind: "recovery", Name: policy.Name, Generation: policy.Generation,
		ExpectedRevision: 1, PolicyID: policy.Digest(), RecoveryStep: step,
		RecoveryNotBefore: proof.StartedAt, RecoveryProof: raw}
}
