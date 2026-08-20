package stage6verify

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

func verifyClaimCorpus(proof claimEvidence, inputs []claimInputEvidence,
	rejections []claimRejectionEvidence,
) bool {
	if len(inputs) == 0 || uint32(len(inputs)) != proof.InputLength {
		return false
	}
	leaves := make([][32]byte, len(inputs))
	for index, input := range inputs {
		if input.Ordinal != uint32(index) || input.Commitment == [32]byte{} || input.AdmissionDigest == [32]byte{} {
			return false
		}
		out := binary.BigEndian.AppendUint32([]byte{0}, input.Ordinal)
		out = append(out, input.Commitment[:]...)
		leaves[index] = sha256.Sum256(append(out, input.AdmissionDigest[:]...))
	}
	if claimMerkleRoot(leaves) != proof.InputRoot || len(rejections) != int(proof.RejectionLength) {
		return false
	}
	rejectionLeaves := make([][32]byte, len(rejections))
	for index, rejection := range rejections {
		if rejection.Ordinal >= proof.InputLength || rejection.Commitment == [32]byte{} || rejection.Reason == "" {
			return false
		}
		out := binary.BigEndian.AppendUint32([]byte{3}, rejection.Ordinal)
		out = append(out, rejection.Commitment[:]...)
		out = claimText(out, rejection.Reason)
		rejectionLeaves[index] = sha256.Sum256(out)
	}
	rejectionRoot := sha256.Sum256([]byte{2})
	if len(rejectionLeaves) > 0 {
		rejectionRoot = claimMerkleRoot(rejectionLeaves)
	}
	if rejectionRoot != proof.RejectionRoot {
		return false
	}
	for _, claim := range proof.Claims {
		if claim.Ordinal >= uint32(len(inputs)) || inputs[claim.Ordinal].Commitment != claim.Commitment ||
			inputs[claim.Ordinal].AdmissionDigest != claim.AdmissionDigest {
			return false
		}
	}
	claims := append([]claimReveal(nil), proof.Claims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].Ordinal < claims[j].Ordinal })
	if len(rejections) != len(claims)-1 {
		return false
	}
	for index, rejection := range rejections {
		loser := claims[index+1]
		if rejection.Ordinal != loser.Ordinal || rejection.Commitment != loser.Commitment ||
			rejection.Reason != "ordered-collision" {
			return false
		}
	}
	return true
}

func claimMerkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	left, right := claimMerkleRoot(leaves[:split]), claimMerkleRoot(leaves[split:])
	return claimNode(left, right)
}
