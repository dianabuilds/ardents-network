package state

// Proof returns the canonical inclusion path for values[index].
func epochCommitmentProof(values [][]byte, index int, emptyTag byte) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := splitAt(len(values))
	if index < split {
		return append(epochCommitmentProof(values[:split], index, emptyTag), epochCommitmentRoot(values[split:], emptyTag))
	}
	return append(epochCommitmentProof(values[split:], index-split, emptyTag), epochCommitmentRoot(values[:split], emptyTag))
}

// Verify reports whether siblings prove record at index in the expected tree.
func verifyEpochCommitment(record []byte, index, length uint32, siblings [][32]byte, expected [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	current := epochCommitmentLeaf(record)
	leafIndex, lastIndex := index, length-1
	for _, sibling := range siblings {
		if leafIndex&1 == 1 || leafIndex == lastIndex {
			current = epochCommitmentBranch(sibling, current)
			for leafIndex&1 == 0 && leafIndex != 0 {
				leafIndex >>= 1
				lastIndex >>= 1
			}
		} else {
			current = epochCommitmentBranch(current, sibling)
		}
		leafIndex >>= 1
		lastIndex >>= 1
	}
	return lastIndex == 0 && current == expected
}
