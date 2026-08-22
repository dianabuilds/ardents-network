package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// Select returns the domain with the lowest canonical assignment digest.
func selectEpochDomain(network [32]byte, epoch uint64, seed [32]byte, family string, domains []string) (string, error) {
	var selected string
	var selectedDigest [32]byte
	for index, domain := range domains {
		digest := epochAssignmentDigest(network, epoch, seed, family, domain)
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

// Digest returns the canonical assignment commitment for one family/domain pair.
func epochAssignmentDigest(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
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
