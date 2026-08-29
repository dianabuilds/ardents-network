package claim

import (
	"errors"
)

// NewClaimProof builds the canonical unsigned close facts for one ordered,
// bounded Name collision set. The Epoch close authority signs the returned
// fields separately; verification remains the sole acceptance boundary.
func NewClaimProof(network [32]byte, epoch uint64, cutoff int64, claims []Claim) (ClaimProof, error) {
	if network == [32]byte{} || epoch == 0 || cutoff < 0 || len(claims) == 0 || len(claims) > 32 {
		return ClaimProof{}, errors.New("claim close construction input is invalid")
	}
	ordered := append([]Claim(nil), claims...)
	for index := range ordered {
		if index > 0 && ordered[index-1].Ordinal >= ordered[index].Ordinal {
			return ClaimProof{}, errors.New("claim close input ordinals are not ordered")
		}
	}
	leaves := make([][32]byte, len(ordered))
	for index, value := range ordered {
		leaves[index] = claimInputLeaf(value)
	}
	for index := range ordered {
		ordered[index].InputPath = claimTreePath(leaves, index)
	}
	rejectionRoot := emptyClaimRoot()
	if len(ordered) > 1 {
		rejections := make([][32]byte, len(ordered)-1)
		for index, value := range ordered[1:] {
			rejections[index] = orderedCollisionLeaf(value)
		}
		rejectionRoot = claimTreeRoot(rejections)
	}
	return ClaimProof{Network: network, Epoch: epoch, Rule: claimOrderRule, CutoffOffset: cutoff,
		InputRoot: claimTreeRoot(leaves), InputLength: uint32(len(leaves)),
		MaterializationRoot: claimMaterializationLeaf(ordered), MaterializationLength: 1,
		RejectionRoot: rejectionRoot, RejectionLength: uint32(len(ordered) - 1), Claims: ordered}, nil
}

func claimTreePath(leaves [][32]byte, index int) [][32]byte {
	if len(leaves) == 1 {
		return nil
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	if index < split {
		return append(claimTreePath(leaves[:split], index), claimTreeRoot(leaves[split:]))
	}
	return append(claimTreePath(leaves[split:], index-split), claimTreeRoot(leaves[:split]))
}
