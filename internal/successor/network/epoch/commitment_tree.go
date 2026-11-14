package epoch

import "crypto/sha256"

// commitmentNode is one memoized subtree of the canonical View commitment.
// Building the tree once lets every inclusion proof reuse the same subtree
// roots instead of re-hashing all leaves for each proof (audit NW-1).
type commitmentNode struct {
	root        [32]byte
	count       int
	left, right *commitmentNode
}

func buildCommitmentTree(leaves [][32]byte, emptyTag byte) *commitmentNode {
	switch len(leaves) {
	case 0:
		return &commitmentNode{root: sha256.Sum256([]byte{emptyTag})}
	case 1:
		return &commitmentNode{root: leaves[0], count: 1}
	}
	split := splitAt(len(leaves))
	left := buildCommitmentTree(leaves[:split], emptyTag)
	right := buildCommitmentTree(leaves[split:], emptyTag)
	return &commitmentNode{
		root:  epochCommitmentBranch(left.root, right.root),
		count: len(leaves),
		left:  left,
		right: right,
	}
}

// proof returns the canonical inclusion path for leaf index. It follows the
// same split and sibling order as epochCommitmentProof, so the path is
// byte-identical for the same leaves.
func (node *commitmentNode) proof(index int) [][32]byte {
	if node.count <= 1 {
		return nil
	}
	split := splitAt(node.count)
	if index < split {
		return append(node.left.proof(index), node.right.root)
	}
	return append(node.right.proof(index-split), node.left.root)
}

// epochCommitmentProofs builds the commitment tree once and returns the
// canonical inclusion path for every index. Each path equals
// epochCommitmentProof(values, index, emptyTag); the total hashing is O(n)
// instead of O(n²) across all indices.
func epochCommitmentProofs(values [][]byte, emptyTag byte) [][][32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		leaves[index] = epochCommitmentLeaf(value)
	}
	tree := buildCommitmentTree(leaves, emptyTag)
	proofs := make([][][32]byte, len(values))
	for index := range values {
		proofs[index] = tree.proof(index)
	}
	return proofs
}
