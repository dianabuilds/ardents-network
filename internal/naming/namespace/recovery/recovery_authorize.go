package recovery

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// Authorize independently verifies one initiation or cancellation proof.
func (policy RecoveryPolicy) Authorize(proof RecoveryProof) (Authorization, error) {
	if !validPolicy(policy) {
		return Authorization{}, errors.New("invalid Recovery Policy")
	}
	if proof.Operation != "initiate" && proof.Operation != "cancel" {
		return Authorization{}, errors.New("recovery operation is invalid")
	}
	if proof.PolicyDigest != policy.Digest() || proof.OperationID == [32]byte{} || proof.Successor == [32]byte{} ||
		proof.StartedAt <= 0 || proof.CompletesAt-proof.StartedAt != policy.Delay.Milliseconds() ||
		len(proof.Signatures) < int(policy.Threshold) || len(proof.Signatures) > len(policy.Participants) ||
		logicalSize(policy, proof) > maximumPolicyProofBytes {
		return Authorization{}, errors.New("recovery proof does not match the effective policy")
	}
	transcript := policy.Transcript(proof)
	for index, signed := range proof.Signatures {
		if index > 0 && bytes.Compare(proof.Signatures[index-1].Signer[:], signed.Signer[:]) >= 0 {
			return Authorization{}, errors.New("recovery signers are duplicate or non-canonical")
		}
		participant := sort.Search(len(policy.Participants), func(i int) bool {
			return bytes.Compare(policy.Participants[i][:], signed.Signer[:]) >= 0
		})
		if participant == len(policy.Participants) || policy.Participants[participant] != signed.Signer ||
			len(signed.Bytes) != ed25519.SignatureSize ||
			!ed25519.Verify(ed25519.PublicKey(signed.Signer[:]), transcript, signed.Bytes) {
			return Authorization{}, errors.New("recovery signature is invalid")
		}
	}
	authorization := Authorization{Operation: proof.Operation, PolicyDigest: proof.PolicyDigest,
		PolicyRevision: policy.Revision, OperationID: proof.OperationID, Successor: proof.Successor,
		StartedAt: proof.StartedAt, CompletesAt: proof.CompletesAt,
		ValidSigners: uint8(len(proof.Signatures))}
	authorization.seal = authorizationSeal(authorization)
	return authorization, nil
}

func validPolicy(policy RecoveryPolicy) bool {
	name, err := naming.Parse(policy.Name)
	if err != nil || string(name) != policy.Name || policy.Network == [32]byte{} || policy.Generation == 0 ||
		policy.Revision == 0 || policy.CurrentAuthority == [32]byte{} || policy.Threshold < 2 ||
		int(policy.Threshold) > len(policy.Participants) || len(policy.Participants) > 8 ||
		policy.Delay < 72*time.Hour || policy.Delay > 30*24*time.Hour {
		return false
	}
	for index, participant := range policy.Participants {
		if participant == [32]byte{} || participant == policy.CurrentAuthority ||
			(index > 0 && bytes.Compare(policy.Participants[index-1][:], participant[:]) >= 0) {
			return false
		}
	}
	return true
}

func logicalSize(policy RecoveryPolicy, proof RecoveryProof) int {
	return 32 + len(policy.Name) + 90 + len(policy.Participants)*32 + len(proof.Operation) +
		len(proof.Signatures)*(32+ed25519.SignatureSize)
}
