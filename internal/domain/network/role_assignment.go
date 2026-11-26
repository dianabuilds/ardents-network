package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// RoleAssignmentDigest retains the accepted assignment transcript verbatim.
// Its historical identifier is a commitment obligation, not a new domain name.
func RoleAssignmentDigest(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
	encoded := append([]byte("ardents-h3-role-domain-v1\x00"), network[:]...)
	encoded = binary.BigEndian.AppendUint64(encoded, epoch)
	encoded = append(encoded, seed[:]...)
	encoded = append(encoded, family...)
	encoded = append(encoded, domain...)
	return sha256.Sum256(encoded)
}

// AssignRoleDomain selects the lowest commitment, refusing a tie involving the
// selected digest. The format owner supplies its bounded canonical domain set.
func AssignRoleDomain(network [32]byte, epoch uint64, seed [32]byte, family string, domains []string) (string, error) {
	var selected string
	var selectedDigest [32]byte
	for index, domain := range domains {
		digest := RoleAssignmentDigest(network, epoch, seed, family, domain)
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

// DomainSummaries assigns each accepted family as a whole, including every
// domain with zero members. Returned totals do not alias the View.
func (view CandidateView) DomainSummaries(network [32]byte, epoch uint64, seed [32]byte, domains []string) (map[string][2]uint32, error) {
	computed := make(map[string][2]uint32, len(domains))
	for _, domain := range domains {
		computed[domain] = [2]uint32{}
	}
	for family, summary := range view.families {
		domain, err := AssignRoleDomain(network, epoch, seed, family, domains)
		if err != nil {
			return nil, err
		}
		current := computed[domain]
		current[0] += summary[0]
		current[1] += summary[1]
		computed[domain] = current
	}
	return computed, nil
}
