package route

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"time"
)

func independentlySelect(input Case) ([4]Candidate, error) {
	var result [4]Candidate
	excludedIDs, excludedFamilies, excludedDomains := map[[32]byte]bool{}, map[string]bool{}, map[string]bool{}
	for _, value := range input.ExcludedIdentities {
		excludedIDs[value] = true
	}
	for _, value := range input.ExcludedFamilies {
		excludedFamilies[value] = true
	}
	for _, value := range input.ExcludedDomains {
		excludedDomains[value] = true
	}
	at := time.Unix(input.SelectionAt, 0).UTC()
	for index, role := range roles {
		var rank [32]byte
		found := false
		for _, candidate := range input.Candidates {
			if candidate.Domain != role || candidate.Capacity == 0 || candidate.NodeID == [32]byte{} ||
				candidate.PublicKey == [32]byte{} || candidate.Family == "" || !literalEndpoint(candidate.Endpoint) ||
				excludedIDs[candidate.NodeID] || excludedFamilies[candidate.Family] || excludedDomains[candidate.Domain] ||
				at.Before(time.Unix(candidate.ValidFrom, 0)) || !at.Before(time.Unix(candidate.ValidUntil, 0)) {
				continue
			}
			candidateRank := rankCandidate(input.SelectionSeed, role, candidate.NodeID)
			if !found || bytes.Compare(candidateRank[:], rank[:]) < 0 {
				result[index], rank, found = candidate, candidateRank, true
			}
		}
		if !found {
			return result, errors.New("frozen Candidate View cannot fill every Route position")
		}
	}
	identities, keys, families, endpoints := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}, map[string]bool{}
	for _, candidate := range result {
		if identities[candidate.NodeID] || keys[candidate.PublicKey] || families[candidate.Family] || endpoints[candidate.Endpoint] {
			return result, errors.New("independently selected Route reuses identity, key, family, or endpoint")
		}
		identities[candidate.NodeID], keys[candidate.PublicKey] = true, true
		families[candidate.Family], endpoints[candidate.Endpoint] = true, true
	}
	return result, nil
}

func rankCandidate(seed [32]byte, role string, identity [32]byte) [32]byte {
	value := make([]byte, 0, 28+32+len(role)+32)
	value = append(value, "ardents-h3-route-select-v1\x00"...)
	value = append(value, seed[:]...)
	value = append(value, role...)
	value = append(value, identity[:]...)
	return sha256.Sum256(value)
}
