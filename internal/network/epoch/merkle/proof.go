package merkle

// Proof returns the canonical inclusion path for values[index].
func Proof(values [][]byte, index int, emptyTag byte) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := splitAt(len(values))
	if index < split {
		return append(Proof(values[:split], index, emptyTag), Root(values[split:], emptyTag))
	}
	return append(Proof(values[split:], index-split, emptyTag), Root(values[:split], emptyTag))
}

// Verify reports whether siblings prove record at index in the expected tree.
func Verify(record []byte, index, length uint32, siblings [][32]byte, expected [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	current := Leaf(record)
	leafIndex, lastIndex := index, length-1
	for _, sibling := range siblings {
		if leafIndex&1 == 1 || leafIndex == lastIndex {
			current = branch(sibling, current)
			for leafIndex&1 == 0 && leafIndex != 0 {
				leafIndex >>= 1
				lastIndex >>= 1
			}
		} else {
			current = branch(current, sibling)
		}
		leafIndex >>= 1
		lastIndex >>= 1
	}
	return lastIndex == 0 && current == expected
}
