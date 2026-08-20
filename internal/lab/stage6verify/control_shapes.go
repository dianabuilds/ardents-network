package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"time"
)

func verifyControlExchanges(evidence controlRoleEvidence, before, after []decodedRecord, secret [32]byte) bool {
	kinds := []string{"claim", "renew", "record", "release", "transfer", "delegate",
		"policy", "policy", "recovery", "recovery", "recovery", "recovery"}
	for index, kind := range kinds {
		exchange, output := evidence.Exchanges[index], after[index]
		transitionOK := kind == "claim" && verifyControlClaim(evidence, exchange.Operation, output) ||
			kind != "claim" && verifyControlTransition(exchange, before, output, evidence.Network)
		if exchange.Operation.Kind != kind || !verifyControlEnvelope(exchange, evidence.Network, secret) ||
			exchange.Result.Class != "accepted" || exchange.Result.Generation != output.Generation ||
			exchange.Result.Revision != output.Revision || !bytes.Equal(exchange.Result.State, encodeRecord(output)) ||
			!transitionOK {
			return false
		}
	}
	return true
}

func verifyControlEnvelope(exchange controlExchangeEvidence, network, secret [32]byte) bool {
	view, challenge := exchange.Operation, exchange.Admission.Challenge
	if exchange.Isolation == [32]byte{} || len(exchange.Envelope) == 0 || view.Network != [32]byte{} ||
		view.Nonce != [32]byte{} || view.Deadline != 0 || challenge.Node != [32]byte{2} || challenge.Network != network ||
		challenge.Surface != controlSurface(view.Kind) || challenge.OperationDigest != view.OperationDigest ||
		view.OperationDigest != verifiedControlDigest(view) || challenge.WorkBits != controlWorkBits(view.Kind) ||
		!validAdmissionProof(admissionCellEvidence{Node: challenge.Node, Network: network,
			Epoch: challenge.Epoch, Now: challenge.IssuedAt, Isolation: exchange.Isolation}, exchange.Admission, secret) {
		return false
	}
	for _, forbidden := range [][]byte{[]byte(view.Name), []byte(view.ParentName), exchange.Isolation[:],
		view.PolicyID[:], []byte(view.RecoveryStep), view.OrderingProof, view.AuthorityProof,
		view.RecoveryPolicy, view.RecoveryProof} {
		if len(forbidden) > 0 && bytes.Contains(exchange.Envelope, forbidden) {
			return false
		}
	}
	return true
}

func verifyControlTransition(exchange controlExchangeEvidence, before []decodedRecord,
	after decodedRecord, network [32]byte,
) bool {
	op := exchange.Operation
	if op.Kind == "delegate" {
		parent, ok := controlRecord(before, op.ParentName)
		want := decodedRecord{Name: op.Name, Lease: "active", Consistency: "current", Recovery: "stable",
			Authority: hex.EncodeToString(op.Authority[:]), Parent: parent.Name, Generation: op.ChildGeneration,
			Revision: 1, ParentGeneration: parent.Generation, LeaseExpires: op.LeaseNotAfter / 1_000,
			GraceExpires: op.LeaseNotAfter/1_000 + 3_600, Continuity: 1}
		return ok && parent.Generation == op.ParentGeneration && parent.Revision == op.ParentRevision &&
			recordsEqual(after, want) && verifyControlSignature(network, parent, op, exchange.Admission.Challenge.IssuedAt)
	}
	current, ok := controlRecord(before, op.Name)
	if !ok || op.Generation != current.Generation || op.ExpectedRevision != current.Revision ||
		after.Name != current.Name || after.Generation != current.Generation || after.Revision != current.Revision+1 {
		return false
	}
	want := current
	want.Revision++
	switch op.Kind {
	case "renew":
		want.Lease, want.LeaseExpires = "active", op.LeaseNotAfter/1_000
		want.GraceExpires = want.LeaseExpires + 3_600
	case "record":
		if op.RecordNotAfter > current.LeaseExpires*1_000 {
			return false
		}
		want.Target = op.Target
	case "release":
		want.Lease, want.LeaseExpires, want.GraceExpires = "released", 0, 0
	case "transfer":
		want.Authority = hex.EncodeToString(op.SuccessorAuthority[:])
	case "policy":
		digest, revision, delay, activates := controlPolicyFields(current, op, exchange.Admission.Challenge.IssuedAt)
		if revision == 0 {
			return false
		}
		want.PendingPolicy, want.PendingPolicyRev = digest, revision
		want.PendingPolicyDelay, want.PolicyActivates = delay.Milliseconds(), activates
	case "recovery":
		if op.PolicyID != current.RecoveryPolicy {
			return false
		}
		switch op.RecoveryStep {
		case "initiate":
			proof, valid := verifyControlRecovery(op, current, exchange.Admission.Challenge.IssuedAt)
			if !valid {
				return false
			}
			want.Recovery, want.RecoveryOperation, want.RecoverySuccessor = "recovery-pending", proof.OperationID, proof.Successor
			want.RecoveryStarted, want.RecoveryExpires = proof.StartedAt, proof.CompletesAt
		case "cancel":
			proof, valid := verifyControlRecovery(op, current, exchange.Admission.Challenge.IssuedAt)
			if !valid || proof.OperationID != current.RecoveryOperation || proof.Successor != current.RecoverySuccessor ||
				proof.StartedAt != current.RecoveryStarted || proof.CompletesAt != current.RecoveryExpires {
				return false
			}
			clearControlRecovery(&want)
		case "complete":
			proof, valid := verifyControlRecovery(op, current, exchange.Admission.Challenge.IssuedAt)
			if !valid || proof.OperationID != current.RecoveryOperation || proof.Successor != current.RecoverySuccessor ||
				proof.StartedAt != current.RecoveryStarted || proof.CompletesAt != current.RecoveryExpires {
				return false
			}
			want.Authority, want.Target, want.Consistency = hex.EncodeToString(proof.Successor[:]), [32]byte{}, "unavailable"
			clearControlRecovery(&want)
		case "resume":
			if current.Consistency != "unavailable" ||
				!verifyControlSignature(network, current, op, exchange.Admission.Challenge.IssuedAt) {
				return false
			}
			want.Target, want.Consistency = op.Target, "current"
		default:
			return false
		}
		return recordsEqual(after, want)
	default:
		return false
	}
	return recordsEqual(after, want) && verifyControlSignature(network, current, op,
		exchange.Admission.Challenge.IssuedAt)
}

func clearControlRecovery(record *decodedRecord) {
	record.Recovery = "stable"
	record.RecoveryOperation, record.RecoverySuccessor = [32]byte{}, [32]byte{}
	record.RecoveryStarted, record.RecoveryExpires = 0, 0
}

func verifyControlSignature(network [32]byte, current decodedRecord, operation controlOperationEvidence,
	issuedAt int64,
) bool {
	public, err := hex.DecodeString(current.Authority)
	if err != nil || len(public) != ed25519.PublicKeySize || len(operation.AuthorityProof) != ed25519.SignatureSize {
		return false
	}
	transcript, ok := controlTransitionTranscript(network, current, operation, issuedAt)
	return ok && ed25519.Verify(ed25519.PublicKey(public), transcript, operation.AuthorityProof)
}

func controlTransitionTranscript(network [32]byte, current decodedRecord, operation controlOperationEvidence,
	issuedAt int64,
) ([]byte, bool) {
	kind, name, authority := operation.Kind, operation.Name, current.Authority
	generation, revision := uint64(0), operation.ExpectedRevision
	successor, target, leaseDuration := "", [32]byte{}, time.Duration(0)
	policyDigest, policyRevision, policyDelay, policyActivates := [32]byte{}, uint64(0), time.Duration(0), int64(0)
	parents := []decodedRecord{}
	switch operation.Kind {
	case "renew":
		leaseDuration = time.Duration(operation.LeaseNotAfter-issuedAt) * time.Millisecond
	case "record":
		kind, target = "publish", operation.Target
	case "release":
	case "transfer":
		successor = hex.EncodeToString(operation.SuccessorAuthority[:])
	case "delegate":
		kind, generation, revision, authority = "claim", operation.ChildGeneration, 0,
			hex.EncodeToString(operation.Authority[:])
		leaseDuration = time.Duration(operation.LeaseNotAfter-issuedAt) * time.Millisecond
		parents = []decodedRecord{current}
	case "policy":
		kind = "schedule-recovery-policy"
		policyDigest, policyRevision, policyDelay, policyActivates = controlPolicyFields(current, operation, issuedAt)
		if policyRevision == 0 {
			return nil, false
		}
	case "recovery":
		if operation.RecoveryStep != "resume" {
			return nil, false
		}
		kind, target = "resume-recovery", operation.Target
	default:
		return nil, false
	}
	out := appendText64(nil, "ardents-name-authority-transition-v1")
	out = append(out, network[:]...)
	out = appendBytes64(out, encodeRecord(current))
	out = appendText64(out, kind)
	out = appendText64(out, name)
	out = binary.BigEndian.AppendUint64(out, generation)
	out = binary.BigEndian.AppendUint32(out, 0)
	out = binary.BigEndian.AppendUint64(out, operation.Generation)
	out = binary.BigEndian.AppendUint64(out, revision)
	out = appendText64(out, authority)
	out = appendText64(out, successor)
	out = append(out, target[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(leaseDuration))
	out = binary.BigEndian.AppendUint64(out, 0)
	out = appendText64(out, "")
	out = append(out, policyDigest[:]...)
	out = binary.BigEndian.AppendUint64(out, policyRevision)
	out = binary.BigEndian.AppendUint64(out, uint64(policyDelay/time.Millisecond))
	out = binary.BigEndian.AppendUint64(out, uint64(policyActivates))
	out = appendText64(out, "")
	out = append(out, make([]byte, 32+8+32+32+8+8)...)
	out = append(out, 0, byte(len(parents)))
	for _, parent := range parents {
		out = appendBytes64(out, encodeRecord(parent))
	}
	return out, true
}

func controlPolicyFields(current decodedRecord, operation controlOperationEvidence,
	issuedAt int64,
) ([32]byte, uint64, time.Duration, int64) {
	if len(operation.RecoveryPolicy) == 0 {
		delay := time.Duration(current.RecoveryPolicyDelay) * time.Millisecond
		if current.RecoveryPolicy == [32]byte{} || delay <= 0 ||
			operation.PolicyNotBefore != issuedAt+delay.Milliseconds() {
			return [32]byte{}, 0, 0, 0
		}
		return [32]byte{}, current.RecoveryPolicyRev + 1, delay, operation.PolicyNotBefore
	}
	var policy controlRecoveryPolicy
	if decodeNestedJSON(operation.RecoveryPolicy, &policy) != nil || policy.Name != operation.Name ||
		!validControlRecoveryPolicy(policy, current, current.RecoveryPolicyRev+1) {
		return [32]byte{}, 0, 0, 0
	}
	if operation.PolicyNotBefore != issuedAt+policy.Delay.Milliseconds() {
		return [32]byte{}, 0, 0, 0
	}
	return controlRecoveryPolicyDigest(policy), policy.Revision, policy.Delay, operation.PolicyNotBefore
}
