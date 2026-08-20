package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

type controlRecoveryPolicy struct {
	Network          [32]byte
	Name             string
	Generation       uint64
	Revision         uint64
	CurrentAuthority [32]byte
	Threshold        uint8
	Participants     [][32]byte
	Delay            time.Duration
}

type controlRecoveryProof struct {
	Operation    string
	PolicyDigest [32]byte
	OperationID  [32]byte
	Successor    [32]byte
	StartedAt    int64
	CompletesAt  int64
	Signatures   []controlRecoverySignature
}

type controlRecoverySignature struct {
	Signer [32]byte
	Bytes  []byte
}

type controlRecoveryEnvelope struct {
	Policy controlRecoveryPolicy `json:"policy"`
	Proof  controlRecoveryProof  `json:"proof"`
}

func verifyControlRecovery(operation controlOperationEvidence, current decodedRecord,
	issuedAt int64,
) (controlRecoveryProof, bool) {
	var envelope controlRecoveryEnvelope
	if decodeNestedJSON(operation.RecoveryProof, &envelope) != nil ||
		!validControlRecoveryPolicy(envelope.Policy, current, current.RecoveryPolicyRev) {
		return controlRecoveryProof{}, false
	}
	policy, proof := envelope.Policy, envelope.Proof
	digest := controlRecoveryPolicyDigest(policy)
	if digest == [32]byte{} || digest != operation.PolicyID || proof.PolicyDigest != digest ||
		proof.OperationID == [32]byte{} || proof.Successor == [32]byte{} ||
		proof.CompletesAt-proof.StartedAt != policy.Delay.Milliseconds() ||
		len(proof.Signatures) < int(policy.Threshold) || len(proof.Signatures) > len(policy.Participants) {
		return controlRecoveryProof{}, false
	}
	wantOperation := "initiate"
	if operation.RecoveryStep == "cancel" {
		wantOperation = "cancel"
	}
	if proof.Operation != wantOperation || !validControlRecoveryBoundary(operation, proof, issuedAt) {
		return controlRecoveryProof{}, false
	}
	transcript := controlRecoveryTranscript(policy, proof)
	for index, signature := range proof.Signatures {
		if index > 0 && bytes.Compare(proof.Signatures[index-1].Signer[:], signature.Signer[:]) >= 0 ||
			!controlRecoveryParticipant(policy.Participants, signature.Signer) ||
			len(signature.Bytes) != ed25519.SignatureSize ||
			!ed25519.Verify(ed25519.PublicKey(signature.Signer[:]), transcript, signature.Bytes) {
			return controlRecoveryProof{}, false
		}
	}
	return proof, true
}

func validControlRecoveryPolicy(policy controlRecoveryPolicy, current decodedRecord, expectedRevision uint64) bool {
	if policy.Network == [32]byte{} || policy.Name != current.Name || policy.Generation != current.Generation ||
		policy.Revision != expectedRevision || hex.EncodeToString(policy.CurrentAuthority[:]) != current.Authority ||
		policy.Threshold < 2 || int(policy.Threshold) > len(policy.Participants) || len(policy.Participants) > 8 ||
		policy.Delay < 72*time.Hour || policy.Delay > 30*24*time.Hour {
		return false
	}
	for index, participant := range policy.Participants {
		if participant == [32]byte{} || participant == policy.CurrentAuthority ||
			index > 0 && bytes.Compare(policy.Participants[index-1][:], participant[:]) >= 0 {
			return false
		}
	}
	return true
}

func controlRecoveryPolicyDigest(policy controlRecoveryPolicy) [32]byte {
	if policy.Name == "" || policy.Revision == 0 {
		return [32]byte{}
	}
	out := appendText32(nil, "ardents-name-recovery-policy-v1")
	out = append(out, policy.Network[:]...)
	out = appendBytes32(out, canonicalNameWire(policy.Name))
	out = binary.BigEndian.AppendUint64(out, policy.Generation)
	out = binary.BigEndian.AppendUint64(out, policy.Revision)
	out = append(out, policy.CurrentAuthority[:]...)
	out = append(out, policy.Threshold, byte(len(policy.Participants)))
	for _, participant := range policy.Participants {
		out = append(out, participant[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(policy.Delay/time.Millisecond))
	return sha256.Sum256(out)
}

func controlRecoveryTranscript(policy controlRecoveryPolicy, proof controlRecoveryProof) []byte {
	domain := "ardents-name-recovery-initiate-v1"
	if proof.Operation == "cancel" {
		domain = "ardents-name-recovery-cancel-v1"
	}
	out := appendText32(nil, domain)
	out = append(out, policy.Network[:]...)
	out = appendBytes32(out, canonicalNameWire(policy.Name))
	out = binary.BigEndian.AppendUint64(out, policy.Generation)
	out = append(out, proof.PolicyDigest[:]...)
	out = append(out, proof.OperationID[:]...)
	out = append(out, proof.Successor[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(proof.StartedAt))
	return binary.BigEndian.AppendUint64(out, uint64(proof.CompletesAt))
}

func validControlRecoveryBoundary(operation controlOperationEvidence, proof controlRecoveryProof,
	issuedAt int64,
) bool {
	switch operation.RecoveryStep {
	case "initiate":
		return operation.RecoveryNotBefore == proof.StartedAt && issuedAt == proof.StartedAt
	case "cancel":
		return operation.RecoveryNotBefore == issuedAt && issuedAt > proof.StartedAt && issuedAt < proof.CompletesAt
	case "complete":
		return operation.RecoveryNotBefore == proof.CompletesAt && issuedAt >= proof.CompletesAt
	}
	return false
}

func controlRecoveryParticipant(participants [][32]byte, signer [32]byte) bool {
	for _, participant := range participants {
		if participant == signer {
			return true
		}
	}
	return false
}
