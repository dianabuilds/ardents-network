package networkstate

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	emptyInputTag     = byte(0x10)
	emptyViewTag      = byte(0x11)
	emptyRejectionTag = byte(0x12)
)

func recordLeaf(value []byte) [32]byte {
	encoded := make([]byte, 5+len(value))
	encoded[0] = 0
	binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
	copy(encoded[5:], value)
	return sha256.Sum256(encoded)
}

func rejectionLeaf(index uint32, code uint16, raw []byte) [32]byte {
	rawDigest := sha256.Sum256(raw)
	encoded := make([]byte, 39)
	encoded[0] = 2
	binary.BigEndian.PutUint32(encoded[1:5], index)
	binary.BigEndian.PutUint16(encoded[5:7], code)
	copy(encoded[7:], rawDigest[:])
	return sha256.Sum256(encoded)
}

func recordMerkleRoot(values [][]byte, emptyTag byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		leaves[index] = recordLeaf(value)
	}
	return hashedMerkleRoot(leaves, emptyTag)
}

func hashedMerkleRoot(leaves [][32]byte, emptyTag byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := merkleSplit(len(leaves))
	return branchHash(
		hashedMerkleRoot(leaves[:split], emptyTag),
		hashedMerkleRoot(leaves[split:], emptyTag),
	)
}

func branchHash(left, right [32]byte) [32]byte {
	encoded := make([]byte, 65)
	encoded[0] = 1
	copy(encoded[1:33], left[:])
	copy(encoded[33:], right[:])
	return sha256.Sum256(encoded)
}

func merkleSplit(length int) int {
	split := 1
	for split<<1 < length {
		split <<= 1
	}
	return split
}

func verifyProof(record []byte, index, length uint32, siblings [][32]byte, expected [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	current := recordLeaf(record)
	leafIndex, lastIndex := index, length-1
	for _, sibling := range siblings {
		if leafIndex&1 == 1 || leafIndex == lastIndex {
			current = branchHash(sibling, current)
			for leafIndex&1 == 0 && leafIndex != 0 {
				leafIndex >>= 1
				lastIndex >>= 1
			}
		} else {
			current = branchHash(current, sibling)
		}
		leafIndex >>= 1
		lastIndex >>= 1
	}
	return lastIndex == 0 && current == expected
}
