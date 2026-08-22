package state

import (
	"crypto/sha256"
	"encoding/binary"
)

// Leaf returns the canonical hash of one length-delimited record.
func epochCommitmentLeaf(value []byte) [32]byte {
	encoded := make([]byte, 5+len(value))
	binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
	copy(encoded[5:], value)
	return sha256.Sum256(encoded)
}

// RejectionLeaf commits one rejected input index, code, and raw digest.
func epochRejectionLeaf(index uint32, code uint16, raw []byte) [32]byte {
	rawDigest := sha256.Sum256(raw)
	encoded := make([]byte, 39)
	encoded[0] = 2
	binary.BigEndian.PutUint32(encoded[1:5], index)
	binary.BigEndian.PutUint16(encoded[5:7], code)
	copy(encoded[7:], rawDigest[:])
	return sha256.Sum256(encoded)
}

// Root returns the canonical root of raw records.
func epochCommitmentRoot(values [][]byte, emptyTag byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		leaves[index] = epochCommitmentLeaf(value)
	}
	return epochHashedCommitmentRoot(leaves, emptyTag)
}

// HashedRoot returns the canonical root of pre-hashed leaves.
func epochHashedCommitmentRoot(leaves [][32]byte, emptyTag byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := splitAt(len(leaves))
	return epochCommitmentBranch(epochHashedCommitmentRoot(leaves[:split], emptyTag), epochHashedCommitmentRoot(leaves[split:], emptyTag))
}

func epochCommitmentBranch(left, right [32]byte) [32]byte {
	encoded := make([]byte, 65)
	encoded[0] = 1
	copy(encoded[1:33], left[:])
	copy(encoded[33:], right[:])
	return sha256.Sum256(encoded)
}

func splitAt(length int) int {
	split := 1
	for split<<1 < length {
		split <<= 1
	}
	return split
}
