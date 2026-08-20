//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

type Operation string

const (
	Initiate Operation = "initiate"
	Cancel   Operation = "cancel"
)

type Outcome string

const (
	Authorized Outcome = "authorized"
	Denied     Outcome = "denied"
)

type Policy struct {
	Network          [32]byte
	Name             string
	Generation       uint64
	Revision         uint64
	CurrentAuthority [32]byte
	Threshold        uint8
	Participants     [][32]byte
	Delay            time.Duration
}

type Signature struct {
	Signer [32]byte
	Bytes  []byte
}

type Proof struct {
	Operation    Operation
	PolicyDigest [32]byte
	OperationID  [32]byte
	Successor    [32]byte
	StartedAt    int64
	CompletesAt  int64
	Signatures   []Signature
}

type Result struct {
	Outcome      Outcome
	ValidSigners uint8
	Reason       string
}

func Authorize(policy Policy, proof Proof) Result {
	if !validPolicy(policy) {
		return Result{Outcome: Denied, Reason: "invalid-policy"}
	}
	if proof.Operation != Initiate && proof.Operation != Cancel {
		return Result{Outcome: Denied, Reason: "invalid-operation"}
	}
	if proof.PolicyDigest != PolicyDigest(policy) || proof.OperationID == [32]byte{} ||
		proof.Successor == [32]byte{} || proof.StartedAt <= 0 || proof.CompletesAt <= proof.StartedAt ||
		proof.CompletesAt-proof.StartedAt != policy.Delay.Milliseconds() ||
		len(proof.Signatures) < int(policy.Threshold) || len(proof.Signatures) > len(policy.Participants) {
		return Result{Outcome: Denied, Reason: "invalid-proof"}
	}
	transcript := RecoveryTranscript(policy, proof)
	for i, signed := range proof.Signatures {
		if i > 0 && bytes.Compare(proof.Signatures[i-1].Signer[:], signed.Signer[:]) >= 0 {
			return Result{Outcome: Denied, Reason: "duplicate-or-reordered-signer"}
		}
		if !knownParticipant(policy.Participants, signed.Signer) || len(signed.Bytes) != ed25519.SignatureSize ||
			!ed25519.Verify(ed25519.PublicKey(signed.Signer[:]), transcript, signed.Bytes) {
			return Result{Outcome: Denied, Reason: "invalid-signature"}
		}
	}
	return Result{Outcome: Authorized, ValidSigners: uint8(len(proof.Signatures))}
}

func PolicyDigest(policy Policy) [32]byte {
	out := appendText(nil, "ardents-name-recovery-policy-v1")
	out = append(out, policy.Network[:]...)
	out = appendText(out, policy.Name)
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

func RecoveryTranscript(policy Policy, proof Proof) []byte {
	domain := "ardents-name-recovery-initiate-v1"
	if proof.Operation == Cancel {
		domain = "ardents-name-recovery-cancel-v1"
	}
	out := appendText(nil, domain)
	out = append(out, policy.Network[:]...)
	out = appendText(out, policy.Name)
	out = binary.BigEndian.AppendUint64(out, policy.Generation)
	out = append(out, proof.PolicyDigest[:]...)
	out = append(out, proof.OperationID[:]...)
	out = append(out, proof.Successor[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(proof.StartedAt))
	return binary.BigEndian.AppendUint64(out, uint64(proof.CompletesAt))
}

func validPolicy(policy Policy) bool {
	name, err := naming.Parse(policy.Name)
	if err != nil || string(name) != policy.Name || policy.Network == [32]byte{} ||
		policy.Generation == 0 || policy.Revision == 0 || policy.CurrentAuthority == [32]byte{} ||
		policy.Threshold < 2 || int(policy.Threshold) > len(policy.Participants) ||
		len(policy.Participants) > 8 || policy.Delay < 72*time.Hour || policy.Delay > 30*24*time.Hour {
		return false
	}
	for i, participant := range policy.Participants {
		if participant == [32]byte{} || participant == policy.CurrentAuthority ||
			(i > 0 && bytes.Compare(policy.Participants[i-1][:], participant[:]) >= 0) {
			return false
		}
	}
	return true
}

func knownParticipant(participants [][32]byte, signer [32]byte) bool {
	index := sortSearch(participants, signer)
	return index < len(participants) && participants[index] == signer
}

func sortSearch(values [][32]byte, target [32]byte) int {
	left, right := 0, len(values)
	for left < right {
		middle := int(uint(left+right) >> 1)
		if bytes.Compare(values[middle][:], target[:]) < 0 {
			left = middle + 1
		} else {
			right = middle
		}
	}
	return left
}

func appendText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
