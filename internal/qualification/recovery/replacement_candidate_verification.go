package recovery

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"net"
	"sort"
)

var replacementRoles = [...]string{"initiator", "introduction", "rendezvous", "responder"}

func verifyReplacementCandidates(values []replacementCandidate) (map[string][]replacementCandidate, error) {
	if len(values) != 12 {
		return nil, errors.New("S4.2 requires exactly three candidates per Route role")
	}
	result := make(map[string][]replacementCandidate, len(replacementRoles))
	identities, keys, families, endpoints := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}, map[string]bool{}
	for _, candidate := range values {
		host, _, err := net.SplitHostPort(candidate.Endpoint)
		if !replacementRole(candidate.Role) || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} ||
			candidate.Family == "" || err != nil || net.ParseIP(host) == nil || identities[candidate.NodeID] ||
			keys[candidate.PublicKey] || families[candidate.Family] || endpoints[candidate.Endpoint] {
			return nil, errors.New("finite alternate candidate identity is invalid")
		}
		identities[candidate.NodeID], keys[candidate.PublicKey] = true, true
		families[candidate.Family], endpoints[candidate.Endpoint] = true, true
		result[candidate.Role] = append(result[candidate.Role], candidate)
	}
	for _, role := range replacementRoles {
		if len(result[role]) != 3 {
			return nil, errors.New("S4.2 candidate role cardinality is invalid")
		}
	}
	return result, nil
}

func layeredCandidates(byRole map[string][]replacementCandidate, seed [32]byte,
	proposal int) (map[string]replacementCandidate, error) {
	if proposal < 0 || proposal > 3 {
		return nil, errors.New("recovery proposal is outside the layered policy")
	}
	indexes := [][4]int{{0, 0, 0, 0}, {1, 1, 0, 1}, {2, 2, 2, 2}, {1, 1, 2, 1}}
	result := make(map[string]replacementCandidate, len(replacementRoles))
	for roleIndex, role := range replacementRoles {
		eligible := orderedReplacementCandidates(byRole[role], seed, role)
		result[role] = eligible[indexes[proposal][roleIndex]]
	}
	return result, nil
}

func layeredExcluded(byRole map[string][]replacementCandidate, seed [32]byte, proposal int) [][32]byte {
	var result [][32]byte
	for roleIndex, role := range replacementRoles {
		eligible := orderedReplacementCandidates(byRole[role], seed, role)
		count := 0
		switch proposal {
		case 1:
			if role != "rendezvous" {
				count = 1
			}
		case 2:
			count = 2
		case 3:
			count = 1
			if roleIndex == 2 {
				count = 2
			}
		}
		for index := range count {
			result = append(result, eligible[index].NodeID)
		}
	}
	return result
}

func orderedReplacementCandidates(values []replacementCandidate, seed [32]byte, role string) []replacementCandidate {
	result := append([]replacementCandidate(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftRank := replacementRank(seed, role, result[left].NodeID)
		rightRank := replacementRank(seed, role, result[right].NodeID)
		return bytes.Compare(leftRank[:], rightRank[:]) < 0
	})
	return result
}

func replacementRank(seed [32]byte, role string, identity [32]byte) [32]byte {
	value := make([]byte, 0, 28+32+len(role)+32)
	value = append(value, "ardents-h3-route-select-v1\x00"...)
	value = append(value, seed[:]...)
	value = append(value, role...)
	value = append(value, identity[:]...)
	return sha256.Sum256(value)
}

func replacementRole(wanted string) bool {
	for _, role := range replacementRoles {
		if wanted == role {
			return true
		}
	}
	return false
}
