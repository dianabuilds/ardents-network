package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
)

type claimEvidence struct {
	Network                [32]byte
	Epoch                  uint64
	Rule                   string
	CutoffOffset           int64
	InputRoot              [32]byte
	InputLength            uint32
	MaterializationRoot    [32]byte
	MaterializationLength  uint32
	RejectionRoot          [32]byte
	RejectionLength        uint32
	MaterializationOrdinal uint32
	MaterializationPath    [][32]byte
	Claims                 []claimReveal
	SignerIDs              [][32]byte
	Signatures             [][]byte
	AlternateSets          []claimEvidence
}

type claimReveal struct {
	Ordinal         uint32
	Name            string
	Secret          [32]byte
	Authority       [32]byte
	Commitment      [32]byte
	AdmissionDigest [32]byte
	InputPath       [][32]byte
	Signature       [64]byte
}

type claimPolicy struct {
	network       [32]byte
	minimumEpoch  uint64
	maximumClaims uint32
	threshold     int
	authorities   map[[32]byte]ed25519.PublicKey
}

type claimTraceEvidence struct {
	Primary    claimEvidence
	Inputs     []claimInputEvidence
	Rejections []claimRejectionEvidence
	Hostile    []claimEvidence
}

type claimInputEvidence struct {
	Ordinal         uint32
	Commitment      [32]byte
	AdmissionDigest [32]byte
}

type claimRejectionEvidence struct {
	Ordinal    uint32
	Commitment [32]byte
	Reason     string
}

func verifyClaimTrace(trace traceRecord) bool {
	var evidence claimTraceEvidence
	policy, ok := claimPolicyFromTrace(trace, &evidence)
	if !ok {
		return false
	}
	proof := evidence.Primary
	if !verifyClaimCorpus(proof, evidence.Inputs, evidence.Rejections) {
		return false
	}
	outcome, winner, losers := evaluateClaim(policy, proof)
	switch trace.Cell {
	case "C4":
		return len(evidence.Hostile) == 0 && outcome == "accepted" && winner == 0 && equalUint32(losers, []uint32{1}) &&
			equalStrings(trace.Fields, []string{"accepted", "ordered-collision"})
	case "C5":
		return len(evidence.Hostile) == 0 && outcome == "conflict" && equalStrings(trace.Fields, []string{"conflict"})
	case "C6":
		want := []string{"unavailable", "unavailable", "fork", "fork", "conflict"}
		return outcome == "accepted" && equalStrings(trace.Fields, want) &&
			equalStrings(evaluateHostileClaims(policy, evidence.Hostile), want)
	}
	return false
}

func claimPolicyFromTrace(trace traceRecord, evidence *claimTraceEvidence) (claimPolicy, bool) {
	if decodeNestedJSON(trace.Auxiliary, evidence) != nil || len(trace.Values) != 3 || len(trace.Input) == 0 || len(trace.Input)%64 != 0 {
		return claimPolicy{}, false
	}
	proof := evidence.Primary
	policy := claimPolicy{network: proof.Network, minimumEpoch: uint64(trace.Values[0]),
		maximumClaims: uint32(trace.Values[1]), threshold: int(trace.Values[2]), authorities: make(map[[32]byte]ed25519.PublicKey)}
	for offset := 0; offset < len(trace.Input); offset += 64 {
		var id [32]byte
		copy(id[:], trace.Input[offset:offset+32])
		public := append(ed25519.PublicKey(nil), trace.Input[offset+32:offset+64]...)
		if sha256.Sum256(public) != id || offset > 0 && bytes.Compare(trace.Input[offset-64:offset], trace.Input[offset:offset+64]) >= 0 {
			return claimPolicy{}, false
		}
		policy.authorities[id] = public
	}
	return policy, true
}

func verifyNamespaceForkTrace(trace traceRecord) bool {
	var evidence claimTraceEvidence
	policy, ok := claimPolicyFromTrace(trace, &evidence)
	if !ok || len(evidence.Hostile) != 0 {
		return false
	}
	outcome, _, _ := evaluateClaim(policy, evidence.Primary)
	return outcome == "fork" && equalStrings(trace.Fields,
		[]string{"fork-unresolved", "different-rule", "no-local-selection"})
}

func evaluateClaim(policy claimPolicy, proof claimEvidence) (string, uint32, []uint32) {
	if !validClaimPolicy(policy, proof) || !validClaimStatement(policy, proof) {
		return "unavailable", 0, nil
	}
	if proof.Rule != "ardents-name-claim-order-v1" {
		return "fork", 0, nil
	}
	if len(proof.AlternateSets) > 1 {
		return "unavailable", 0, nil
	}
	if len(proof.AlternateSets) == 1 {
		alternate := proof.AlternateSets[0]
		if alternate.Network != proof.Network || alternate.Epoch != proof.Epoch ||
			!validClaimClose(alternate) || !validClaimStatement(policy, alternate) {
			return "unavailable", 0, nil
		}
		if sameClaimClose(proof, alternate) {
			return "unavailable", 0, nil
		}
		return "fork", 0, nil
	}
	claims := proof.Claims
	for index, claim := range claims {
		if !validClaimReveal(proof, claim) {
			return "conflict", 0, nil
		}
		if index > 0 && claim.Name != claims[0].Name {
			return "conflict", 0, nil
		}
		if index > 0 && claim.Ordinal <= claims[index-1].Ordinal {
			return "fork", 0, nil
		}
	}
	if !claimInclusion(claimMaterializationLeaf(claims), proof.MaterializationOrdinal,
		proof.MaterializationLength, proof.MaterializationPath, proof.MaterializationRoot) {
		return "unavailable", 0, nil
	}
	losers := make([]uint32, 0, len(claims)-1)
	for _, claim := range claims[1:] {
		losers = append(losers, claim.Ordinal)
	}
	return "accepted", claims[0].Ordinal, losers
}

func validClaimPolicy(policy claimPolicy, proof claimEvidence) bool {
	return policy.network != [32]byte{} && policy.minimumEpoch > 0 && policy.maximumClaims > 0 &&
		policy.maximumClaims <= 32 && policy.threshold >= 2 && policy.threshold <= len(policy.authorities) &&
		len(policy.authorities) <= 16 && proof.Network == policy.network && proof.Epoch >= policy.minimumEpoch &&
		len(proof.Claims) > 0 && uint32(len(proof.Claims)) <= policy.maximumClaims &&
		len(proof.MaterializationPath) <= 32 && validClaimClose(proof)
}

func validClaimClose(proof claimEvidence) bool {
	empty := sha256.Sum256([]byte{2})
	return proof.Network != [32]byte{} && proof.Epoch > 0 && proof.Rule != "" && proof.CutoffOffset >= 0 &&
		proof.InputRoot != [32]byte{} && proof.InputLength > 0 && proof.MaterializationRoot != [32]byte{} &&
		proof.MaterializationLength > 0 && proof.MaterializationOrdinal < proof.MaterializationLength &&
		proof.RejectionRoot != [32]byte{} && (proof.RejectionLength != 0 || proof.RejectionRoot == empty)
}

func validClaimStatement(policy claimPolicy, proof claimEvidence) bool {
	if len(proof.SignerIDs) < policy.threshold || len(proof.SignerIDs) != len(proof.Signatures) ||
		len(proof.SignerIDs) > len(policy.authorities) {
		return false
	}
	transcript := claimStatement(proof)
	for index, id := range proof.SignerIDs {
		public, ok := policy.authorities[id]
		if !ok || index > 0 && bytes.Compare(proof.SignerIDs[index-1][:], id[:]) >= 0 ||
			len(proof.Signatures[index]) != ed25519.SignatureSize || !ed25519.Verify(public, transcript, proof.Signatures[index]) {
			return false
		}
	}
	return true
}
