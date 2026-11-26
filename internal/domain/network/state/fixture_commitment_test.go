package state_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// These builders deliberately remain independent from State's verifier so
// malformed commitments cannot be concealed by shared fixture logic.
func fixtureCommitmentLeaf(value []byte) [32]byte {
	encoded := make([]byte, 5+len(value))
	binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
	copy(encoded[5:], value)
	return sha256.Sum256(encoded)
}

func fixtureRejectionLeaf(index uint32, code uint16, raw []byte) [32]byte {
	rawDigest := sha256.Sum256(raw)
	encoded := make([]byte, 39)
	encoded[0] = 2
	binary.BigEndian.PutUint32(encoded[1:5], index)
	binary.BigEndian.PutUint16(encoded[5:7], code)
	copy(encoded[7:], rawDigest[:])
	return sha256.Sum256(encoded)
}

func fixtureCommitmentRoot(values [][]byte, emptyTag byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		leaves[index] = fixtureCommitmentLeaf(value)
	}
	return fixtureHashedCommitmentRoot(leaves, emptyTag)
}

func fixtureHashedCommitmentRoot(leaves [][32]byte, emptyTag byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := fixtureCommitmentSplit(len(leaves))
	return fixtureCommitmentBranch(fixtureHashedCommitmentRoot(leaves[:split], emptyTag), fixtureHashedCommitmentRoot(leaves[split:], emptyTag))
}

func fixtureCommitmentProof(values [][]byte, index int, emptyTag byte) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := fixtureCommitmentSplit(len(values))
	if index < split {
		return append(fixtureCommitmentProof(values[:split], index, emptyTag), fixtureCommitmentRoot(values[split:], emptyTag))
	}
	return append(fixtureCommitmentProof(values[split:], index-split, emptyTag), fixtureCommitmentRoot(values[:split], emptyTag))
}

func fixtureCommitmentBranch(left, right [32]byte) [32]byte {
	encoded := make([]byte, 65)
	encoded[0] = 1
	copy(encoded[1:33], left[:])
	copy(encoded[33:], right[:])
	return sha256.Sum256(encoded)
}

func fixtureCommitmentSplit(length int) int {
	split := 1
	for split<<1 < length {
		split <<= 1
	}
	return split
}

func fixtureAssignmentDomain(network [32]byte, epoch uint64, seed [32]byte, family string, domains []string) (string, error) {
	var selected string
	var selectedDigest [32]byte
	for index, domain := range domains {
		digest := fixtureAssignmentDigest(network, epoch, seed, family, domain)
		if index > 0 && digest == selectedDigest {
			return "", errors.New("role assignment digest tie")
		}
		if selected == "" || bytes.Compare(digest[:], selectedDigest[:]) < 0 {
			selected, selectedDigest = domain, digest
		}
	}
	if selected == "" {
		return "", errors.New("role assignment requires a domain")
	}
	return selected, nil
}

func fixtureAssignmentDigest(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
	encoded := make([]byte, 0, 27+32+8+32+len(family)+len(domain))
	encoded = append(encoded, []byte("ardents-h3-role-domain-v1\x00")...)
	encoded = append(encoded, network[:]...)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], epoch)
	encoded = append(encoded, number[:]...)
	encoded = append(encoded, seed[:]...)
	encoded = append(encoded, family...)
	encoded = append(encoded, domain...)
	return sha256.Sum256(encoded)
}
