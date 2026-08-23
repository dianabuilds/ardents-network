package route

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

var routeRoles = [...]string{"initiator", "introduction", "rendezvous", "responder"}

type selection struct {
	seed               [32]byte
	at                 time.Time
	excludedIdentities [][32]byte
	excludedFamilies   []string
	excludedDomains    []string
}

type position struct {
	role                           string
	nodeID, publicKey              [32]byte
	family, endpoint, domain       string
	capacity                       uint16
	validUntil, assignmentNotAfter time.Time
}

// plan is Route-private volatile selection state. It never crosses Route's
// interface: a caller receives only an Attachment byte carrier.
type plan struct {
	networkID  [32]byte
	epoch      uint64
	digest     [32]byte
	profile    string
	validUntil time.Time
	positions  []position
}

func selectRoute(view state.Snapshot, input selection) (plan, error) {
	if input.seed == [32]byte{} || input.at.IsZero() || view.Generation == "" || view.Epoch == 0 ||
		view.NetworkID == [32]byte{} || view.Digest == [32]byte{} || !input.at.Before(view.ValidUntil) ||
		view.Profile != routeProfile || view.ViewRoot == [32]byte{} || view.Freshness != "fresh" ||
		view.Conflicting || view.CandidateCount == 0 || view.CandidateCount > 64 {
		return plan{}, errors.New("route selection input is invalid")
	}
	excludedIDs, excludedFamilies, excludedDomains := exclusions(input)
	result := plan{networkID: view.NetworkID, epoch: view.Epoch, digest: view.Digest, profile: view.Profile,
		validUntil: view.ValidUntil}
	for _, role := range routeRoles {
		candidate, ok := choose(view, role, input, excludedIDs, excludedFamilies, excludedDomains)
		if !ok {
			return plan{}, errors.New("authenticated Candidate View cannot fill every Route position")
		}
		result.positions = append(result.positions, position{role: role, nodeID: candidate.nodeID,
			publicKey: candidate.publicKey, family: candidate.family, endpoint: candidate.endpoint,
			capacity: candidate.capacity, domain: candidate.domain, validUntil: candidate.validUntil,
			assignmentNotAfter: candidate.assignmentNotAfter})
	}
	if err := validatePlan(result); err != nil {
		return plan{}, err
	}
	return result, nil
}

func exclusions(input selection) (map[[32]byte]bool, map[string]bool, map[string]bool) {
	identities := make(map[[32]byte]bool, len(input.excludedIdentities))
	for _, identity := range input.excludedIdentities {
		identities[identity] = true
	}
	families := make(map[string]bool, len(input.excludedFamilies))
	for _, family := range input.excludedFamilies {
		families[family] = true
	}
	domains := make(map[string]bool, len(input.excludedDomains))
	for _, domain := range input.excludedDomains {
		domains[domain] = true
	}
	return identities, families, domains
}

type candidate struct {
	nodeID, publicKey                         [32]byte
	family, endpoint, domain                  string
	capacity                                  uint16
	validFrom, validUntil, assignmentNotAfter time.Time
}

func choose(view state.Snapshot, role string, input selection, excludedIDs map[[32]byte]bool,
	excludedFamilies, excludedDomains map[string]bool) (candidate, bool) {
	var selected candidate
	var selectedRank [32]byte
	found := false
	for _, value := range view.Candidates[:view.CandidateCount] {
		candidate := candidate{nodeID: value.NodeID, publicKey: value.PublicKey, family: value.Family,
			endpoint: value.Endpoint, capacity: value.Capacity, domain: value.Domain,
			validFrom: value.ValidFrom, validUntil: value.ValidUntil, assignmentNotAfter: value.AssignmentNotAfter}
		if candidate.domain != role || candidate.capacity == 0 || candidate.nodeID == [32]byte{} ||
			candidate.publicKey == [32]byte{} || candidate.family == "" || excludedIDs[candidate.nodeID] ||
			excludedFamilies[candidate.family] || excludedDomains[candidate.domain] || input.at.Before(candidate.validFrom) ||
			!input.at.Before(candidate.validUntil) || !input.at.Before(candidate.assignmentNotAfter) || !literalEndpoint(candidate.endpoint) {
			continue
		}
		rank := candidateRank(input.seed, role, candidate.nodeID)
		if !found || bytes.Compare(rank[:], selectedRank[:]) < 0 {
			selected, selectedRank, found = candidate, rank, true
		}
	}
	return selected, found
}

func candidateRank(seed [32]byte, role string, identity [32]byte) [32]byte {
	value := make([]byte, 0, 37+32+len(role)+32)
	value = append(value, "ardents-interactive-route-select-v1\x00"...)
	value = append(value, seed[:]...)
	value = append(value, role...)
	value = append(value, identity[:]...)
	return sha256.Sum256(value)
}

func literalEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}

func validatePlan(value plan) error {
	if value.networkID == [32]byte{} || value.epoch == 0 || value.digest == [32]byte{} || value.profile != routeProfile ||
		value.validUntil.IsZero() || len(value.positions) != len(routeRoles) {
		return errors.New("route must contain every fixed position")
	}
	identities, keys, families, endpoints := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}, map[string]bool{}
	for index, position := range value.positions {
		if position.role != routeRoles[index] || position.domain != routeRoles[index] || position.capacity == 0 ||
			position.nodeID == [32]byte{} || position.publicKey == [32]byte{} || position.family == "" ||
			position.validUntil.IsZero() || position.assignmentNotAfter.IsZero() || !literalEndpoint(position.endpoint) ||
			identities[position.nodeID] || keys[position.publicKey] || families[position.family] || endpoints[position.endpoint] {
			return errors.New("route position identity, family, domain, or endpoint is invalid")
		}
		identities[position.nodeID], keys[position.publicKey] = true, true
		families[position.family], endpoints[position.endpoint] = true, true
	}
	return nil
}
