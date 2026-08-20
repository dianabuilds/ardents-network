package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
)

type controlClaimProofWire struct {
	Network                []byte                   `json:"network"`
	Epoch                  uint64                   `json:"epoch"`
	Rule                   string                   `json:"rule"`
	CutoffOffset           int64                    `json:"cutoff_offset"`
	InputRoot              []byte                   `json:"input_root"`
	InputLength            uint32                   `json:"input_length"`
	MaterializationRoot    []byte                   `json:"materialization_root"`
	MaterializationLength  uint32                   `json:"materialization_length"`
	RejectionRoot          []byte                   `json:"rejection_root"`
	RejectionLength        uint32                   `json:"rejection_length"`
	MaterializationOrdinal uint32                   `json:"materialization_ordinal"`
	MaterializationPath    [][]byte                 `json:"materialization_path"`
	Claims                 []controlClaimRevealWire `json:"claims"`
	SignerIDs              [][]byte                 `json:"signer_ids"`
	Signatures             [][]byte                 `json:"signatures"`
	AlternateSets          []controlClaimProofWire  `json:"alternate_sets"`
}

type controlClaimRevealWire struct {
	Ordinal         uint32   `json:"ordinal"`
	Name            string   `json:"name"`
	Secret          []byte   `json:"secret"`
	Authority       []byte   `json:"authority"`
	Commitment      []byte   `json:"commitment"`
	AdmissionDigest []byte   `json:"admission_digest"`
	InputPath       [][]byte `json:"input_path"`
	Signature       []byte   `json:"signature"`
}

func verifyControlClaim(evidence controlRoleEvidence, operation controlOperationEvidence,
	after decodedRecord,
) bool {
	var wire controlClaimProofWire
	if len(operation.OrderingProof) == 0 || len(operation.OrderingProof) > 2<<10 ||
		decodeNestedJSON(operation.OrderingProof, &wire) != nil {
		return false
	}
	proof, ok := controlClaimProof(wire)
	if !ok {
		return false
	}
	policy := claimPolicy{network: evidence.Network, minimumEpoch: evidence.ClaimEpoch,
		maximumClaims: evidence.ClaimMaximum, threshold: evidence.ClaimThreshold,
		authorities: make(map[[32]byte]ed25519.PublicKey)}
	for offset := 0; offset < len(evidence.ClaimAuthorities); offset += 64 {
		var id [32]byte
		copy(id[:], evidence.ClaimAuthorities[offset:offset+32])
		public := append(ed25519.PublicKey(nil), evidence.ClaimAuthorities[offset+32:offset+64]...)
		if sha256.Sum256(public) != id || offset > 0 &&
			bytes.Compare(evidence.ClaimAuthorities[offset-64:offset], evidence.ClaimAuthorities[offset:offset+64]) >= 0 {
			return false
		}
		policy.authorities[id] = public
	}
	outcome, winner, losers := evaluateClaim(policy, proof)
	want := decodedRecord{Name: operation.Name, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(operation.Authority[:]), Generation: operation.Generation, Revision: 1,
		LeaseExpires: operation.LeaseNotAfter / 1_000, GraceExpires: operation.LeaseNotAfter/1_000 + 3_600,
		Continuity: 1}
	return outcome == "accepted" && winner == proof.Claims[0].Ordinal && len(losers) == 0 &&
		proof.Claims[0].Name == operation.Name && proof.Claims[0].Authority == operation.Authority &&
		recordsEqual(after, want)
}

func controlClaimProof(wire controlClaimProofWire) (claimEvidence, bool) {
	proof := claimEvidence{Epoch: wire.Epoch, Rule: wire.Rule, CutoffOffset: wire.CutoffOffset,
		InputLength: wire.InputLength, MaterializationLength: wire.MaterializationLength,
		RejectionLength: wire.RejectionLength, MaterializationOrdinal: wire.MaterializationOrdinal,
		Signatures: wire.Signatures}
	if !controlCopy32(&proof.Network, wire.Network) || !controlCopy32(&proof.InputRoot, wire.InputRoot) ||
		!controlCopy32(&proof.MaterializationRoot, wire.MaterializationRoot) ||
		!controlCopy32(&proof.RejectionRoot, wire.RejectionRoot) {
		return claimEvidence{}, false
	}
	for _, raw := range wire.MaterializationPath {
		var value [32]byte
		if !controlCopy32(&value, raw) {
			return claimEvidence{}, false
		}
		proof.MaterializationPath = append(proof.MaterializationPath, value)
	}
	for _, item := range wire.Claims {
		claim := claimReveal{Ordinal: item.Ordinal, Name: item.Name}
		if !controlCopy32(&claim.Secret, item.Secret) || !controlCopy32(&claim.Authority, item.Authority) ||
			!controlCopy32(&claim.Commitment, item.Commitment) ||
			!controlCopy32(&claim.AdmissionDigest, item.AdmissionDigest) || len(item.Signature) != 64 {
			return claimEvidence{}, false
		}
		copy(claim.Signature[:], item.Signature)
		for _, raw := range item.InputPath {
			var value [32]byte
			if !controlCopy32(&value, raw) {
				return claimEvidence{}, false
			}
			claim.InputPath = append(claim.InputPath, value)
		}
		proof.Claims = append(proof.Claims, claim)
	}
	for _, raw := range wire.SignerIDs {
		var value [32]byte
		if !controlCopy32(&value, raw) {
			return claimEvidence{}, false
		}
		proof.SignerIDs = append(proof.SignerIDs, value)
	}
	for _, alternate := range wire.AlternateSets {
		value, ok := controlClaimProof(alternate)
		if !ok {
			return claimEvidence{}, false
		}
		proof.AlternateSets = append(proof.AlternateSets, value)
	}
	return proof, true
}

func controlCopy32(target *[32]byte, raw []byte) bool {
	if len(raw) != 32 {
		return false
	}
	copy(target[:], raw)
	return true
}
