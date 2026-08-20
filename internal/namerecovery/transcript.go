package namerecovery

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// Digest returns the canonical domain-separated policy commitment.
func (policy RecoveryPolicy) Digest() [32]byte {
	if !validPolicy(policy) {
		return [32]byte{}
	}
	name, err := naming.Parse(policy.Name)
	if err != nil {
		return [32]byte{}
	}
	wire, err := naming.EncodeWire(name)
	if err != nil {
		return [32]byte{}
	}
	out := appendText(nil, "ardents-name-recovery-policy-v1")
	out = append(out, policy.Network[:]...)
	out = appendBytes(out, wire)
	out = binary.BigEndian.AppendUint64(out, policy.Generation)
	out = binary.BigEndian.AppendUint64(out, policy.Revision)
	out = append(out, policy.CurrentAuthority[:]...)
	out = append(out, policy.Threshold, byte(len(policy.Participants)))
	for _, participant := range policy.Participants {
		out = append(out, participant[:]...)
	}
	out = binary.BigEndian.AppendUint64(out, uint64(policy.Delay.Milliseconds()))
	return sha256.Sum256(out)
}

// Transcript returns the canonical bytes every participant signs.
func (policy RecoveryPolicy) Transcript(proof Proof) []byte {
	domain := "ardents-name-recovery-initiate-v1"
	if proof.Operation == "cancel" {
		domain = "ardents-name-recovery-cancel-v1"
	}
	name, err := naming.Parse(policy.Name)
	if err != nil {
		return nil
	}
	wire, err := naming.EncodeWire(name)
	if err != nil {
		return nil
	}
	out := appendText(nil, domain)
	out = append(out, policy.Network[:]...)
	out = appendBytes(out, wire)
	out = binary.BigEndian.AppendUint64(out, policy.Generation)
	out = append(out, proof.PolicyDigest[:]...)
	out = append(out, proof.OperationID[:]...)
	out = append(out, proof.Successor[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(proof.StartedAt))
	return binary.BigEndian.AppendUint64(out, uint64(proof.CompletesAt))
}

func appendText(out []byte, value string) []byte { return appendBytes(out, []byte(value)) }

func appendBytes(out, value []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}

func authorizationSeal(value Authorization) [32]byte {
	out := appendText(nil, "ardents-name-recovery-authorization-v1")
	out = appendText(out, value.Operation)
	out = append(out, value.PolicyDigest[:]...)
	out = binary.BigEndian.AppendUint64(out, value.PolicyRevision)
	out = append(out, value.OperationID[:]...)
	out = append(out, value.Successor[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.StartedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(value.CompletesAt))
	out = append(out, value.ValidSigners)
	return sha256.Sum256(out)
}
