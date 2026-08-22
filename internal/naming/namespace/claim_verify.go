package namespace

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// Verify authenticates one exact Epoch close and returns only its proven
// input-ordinal winner or an explicit conflict, fork, or unavailable outcome.
func (order ClaimOrder) Verify(proof ClaimProof) (result, error) {
	if !validOrderInput(order, proof) || !validStatement(order, proof) {
		return result{Outcome: "unavailable"}, errors.New("claim Epoch close is incomplete or invalid")
	}
	if proof.Rule != order.Rule {
		return result{Outcome: "fork"}, nil
	}
	if len(proof.AlternateSets) > 1 {
		return result{Outcome: "unavailable"}, errors.New("claim Epoch alternatives are unbounded")
	}
	if len(proof.AlternateSets) == 1 {
		alternate := proof.AlternateSets[0]
		if alternate.Network != proof.Network || alternate.Epoch != proof.Epoch ||
			!validCloseFields(alternate) || !validStatement(order, alternate) {
			return result{Outcome: "unavailable"}, errors.New("alternate claim Epoch close is invalid")
		}
		if sameClose(proof, alternate) {
			return result{Outcome: "unavailable"}, errors.New("alternate duplicates the primary Epoch close")
		}
		return result{Outcome: "fork"}, nil
	}
	claims := proof.Claims
	for index, claim := range claims {
		name, err := naming.Parse(claim.Name)
		if err != nil || string(name) != claim.Name || claim.Authority == [32]byte{} || claim.Secret == [32]byte{} ||
			claim.AdmissionDigest == [32]byte{} || claim.Ordinal >= proof.InputLength || len(claim.InputPath) > 32 ||
			claim.Commitment != CommitmentFor(proof.Network, proof.Epoch, claim) ||
			!ed25519.Verify(ed25519.PublicKey(claim.Authority[:]),
				RevealTranscript(proof.Network, proof.Epoch, claim), claim.Signature[:]) ||
			!verifyInclusion(claimInputLeaf(claim), claim.Ordinal, proof.InputLength, claim.InputPath, proof.InputRoot) {
			return result{Outcome: "conflict"}, errors.New("claim reveal or input inclusion is invalid")
		}
		if index > 0 && claim.Name != claims[0].Name {
			return result{Outcome: "conflict"}, errors.New("claim materialization contains multiple Names")
		}
		if index > 0 && claim.Ordinal <= claims[index-1].Ordinal {
			return result{Outcome: "fork"}, errors.New("claim input ordinals are duplicate or non-canonical")
		}
	}
	leaf := claimMaterializationLeaf(claims)
	if !verifyInclusion(leaf, proof.MaterializationOrdinal, proof.MaterializationLength,
		proof.MaterializationPath, proof.MaterializationRoot) {
		return result{Outcome: "unavailable"}, errors.New("claim materialization inclusion is invalid")
	}
	rejectionRoot := emptyClaimRoot()
	if len(claims) > 1 {
		rejectionLeaves := make([][32]byte, len(claims)-1)
		for index, claim := range claims[1:] {
			rejectionLeaves[index] = orderedCollisionLeaf(claim)
		}
		rejectionRoot = claimTreeRoot(rejectionLeaves)
	}
	if proof.RejectionLength != uint32(len(claims)-1) || proof.RejectionRoot != rejectionRoot {
		return result{Outcome: "unavailable"}, errors.New("claim rejection materialization is incomplete")
	}
	outcome := result{Outcome: "accepted", WinnerOrdinal: claims[0].Ordinal,
		OperationDigest: claims[0].Commitment}
	for _, claim := range claims[1:] {
		outcome.LoserOrdinals = append(outcome.LoserOrdinals, claim.Ordinal)
	}
	return outcome, nil
}

func validOrderInput(order ClaimOrder, proof ClaimProof) bool {
	return order.Network != [32]byte{} && order.Rule == claimOrderRule && order.MinimumEpoch > 0 &&
		order.MaximumClaims > 0 && order.MaximumClaims <= 32 && order.Threshold >= 2 &&
		order.Threshold <= len(order.Authorities) && len(order.Authorities) <= 16 &&
		proof.Network == order.Network && proof.Epoch >= order.MinimumEpoch && len(proof.Claims) > 0 &&
		uint32(len(proof.Claims)) <= order.MaximumClaims && len(proof.MaterializationPath) <= 32 && validCloseFields(proof)
}

func validCloseFields(proof ClaimProof) bool {
	empty := emptyClaimRoot()
	return proof.Network != [32]byte{} && proof.Epoch > 0 && proof.Rule != "" && proof.CutoffOffset >= 0 &&
		proof.InputRoot != [32]byte{} && proof.InputLength > 0 && proof.MaterializationRoot != [32]byte{} &&
		proof.MaterializationLength > 0 && proof.MaterializationOrdinal < proof.MaterializationLength &&
		proof.RejectionRoot != [32]byte{} && (proof.RejectionLength != 0 || proof.RejectionRoot == empty)
}

func validStatement(order ClaimOrder, proof ClaimProof) bool {
	if len(proof.SignerIDs) < order.Threshold || len(proof.SignerIDs) != len(proof.Signatures) ||
		len(proof.SignerIDs) > len(order.Authorities) {
		return false
	}
	transcript := StatementTranscript(proof)
	for index, id := range proof.SignerIDs {
		if index > 0 && bytes.Compare(proof.SignerIDs[index-1][:], id[:]) >= 0 {
			return false
		}
		public, ok := order.Authorities[id]
		if !ok || len(public) != ed25519.PublicKeySize || sha256.Sum256(public) != id ||
			len(proof.Signatures[index]) != ed25519.SignatureSize ||
			!ed25519.Verify(public, transcript, proof.Signatures[index]) {
			return false
		}
	}
	return true
}

func sameClose(left, right ClaimProof) bool {
	return left.Rule == right.Rule && left.CutoffOffset == right.CutoffOffset &&
		left.InputRoot == right.InputRoot && left.InputLength == right.InputLength &&
		left.MaterializationRoot == right.MaterializationRoot &&
		left.MaterializationLength == right.MaterializationLength &&
		left.RejectionRoot == right.RejectionRoot && left.RejectionLength == right.RejectionLength
}

func claimInputLeaf(claim Claim) [32]byte {
	out := []byte{0}
	out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	out = append(out, claim.AdmissionDigest[:]...)
	return sha256.Sum256(out)
}

func claimMaterializationLeaf(claims []Claim) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	for _, claim := range claims {
		out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
		out = claimText(out, claim.Name)
		out = append(out, claim.Commitment[:]...)
		out = append(out, claim.Authority[:]...)
		out = append(out, claim.AdmissionDigest[:]...)
	}
	return sha256.Sum256(out)
}

func verifyInclusion(leaf [32]byte, index, length uint32, path [][32]byte, root [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	value, fn, sn := leaf, index, length-1
	for _, sibling := range path {
		if fn&1 == 1 || fn == sn {
			value = claimNode(sibling, value)
			for fn&1 == 0 && fn != 0 {
				fn >>= 1
				sn >>= 1
			}
		} else {
			value = claimNode(value, sibling)
		}
		fn >>= 1
		sn >>= 1
	}
	return sn == 0 && value == root
}

func claimNode(left, right [32]byte) [32]byte {
	out := append([]byte{1}, left[:]...)
	return sha256.Sum256(append(out, right[:]...))
}

func orderedCollisionLeaf(claim Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{3}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	out = claimText(out, "ordered-collision")
	return sha256.Sum256(out)
}

func claimTreeRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	return claimNode(claimTreeRoot(leaves[:split]), claimTreeRoot(leaves[split:]))
}

func emptyClaimRoot() [32]byte { return sha256.Sum256([]byte{2}) }
