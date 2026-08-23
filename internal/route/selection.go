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

// Selection is the complete endpoint-local input to one Route choice.
type Selection struct {
	Seed               [32]byte
	At                 time.Time
	ExcludedIdentities [][32]byte
	ExcludedFamilies   []string
	ExcludedDomains    []string
}

// Position is one authenticated Node selected for one fixed Route role.
type Position struct {
	Role      string
	NodeID    [32]byte
	PublicKey [32]byte
	Family    string
	Endpoint  string
	Capacity  uint16
	Domain    string
}

// Plan is one complete endpoint-owned Route selection.
type Plan struct {
	NetworkID          [32]byte
	Generation         string
	Epoch              uint64
	Digest             [32]byte
	Profile            string
	ViewRoot           [32]byte
	Seed               [32]byte
	SelectionAt        int64
	ExcludedIdentities [][32]byte
	ExcludedFamilies   []string
	ExcludedDomains    []string
	Positions          []Position
}

// Select chooses one distinct authenticated Candidate for every frozen role.
func Select(view state.Snapshot, input Selection) (Plan, error) {
	if input.Seed == [32]byte{} || input.At.IsZero() || view.Generation == "" || view.Epoch == 0 ||
		view.NetworkID == [32]byte{} || view.Digest == [32]byte{} || !input.At.Before(view.ValidUntil) ||
		view.Profile != routeProfile || view.ViewRoot == [32]byte{} || view.Freshness != "fresh" ||
		view.Conflicting || view.CandidateCount == 0 || view.CandidateCount > 64 {
		return Plan{}, errors.New("route selection input is invalid")
	}
	excludedIDs, excludedFamilies, excludedDomains := exclusions(input)
	plan := Plan{NetworkID: view.NetworkID, Generation: view.Generation, Epoch: view.Epoch, Digest: view.Digest,
		Profile: view.Profile, ViewRoot: view.ViewRoot, Seed: input.Seed, SelectionAt: input.At.Unix(),
		ExcludedIdentities: append([][32]byte(nil), input.ExcludedIdentities...),
		ExcludedFamilies:   append([]string(nil), input.ExcludedFamilies...),
		ExcludedDomains:    append([]string(nil), input.ExcludedDomains...)}
	for _, role := range routeRoles {
		candidate, ok := choose(view, role, input, excludedIDs, excludedFamilies, excludedDomains)
		if !ok {
			return Plan{}, errors.New("authenticated Candidate View cannot fill every Route position")
		}
		plan.Positions = append(plan.Positions, Position{Role: role, NodeID: candidate.nodeID,
			PublicKey: candidate.publicKey, Family: candidate.family, Endpoint: candidate.endpoint,
			Capacity: candidate.capacity, Domain: candidate.domain})
	}
	if err := Validate(plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func exclusions(input Selection) (map[[32]byte]bool, map[string]bool, map[string]bool) {
	identities := make(map[[32]byte]bool, len(input.ExcludedIdentities))
	for _, identity := range input.ExcludedIdentities {
		identities[identity] = true
	}
	families := make(map[string]bool, len(input.ExcludedFamilies))
	for _, family := range input.ExcludedFamilies {
		families[family] = true
	}
	domains := make(map[string]bool, len(input.ExcludedDomains))
	for _, domain := range input.ExcludedDomains {
		domains[domain] = true
	}
	return identities, families, domains
}

type candidate struct {
	nodeID, publicKey        [32]byte
	family, endpoint, domain string
	capacity                 uint16
	validFrom, validUntil    time.Time
}

func choose(view state.Snapshot, role string, input Selection, excludedIDs map[[32]byte]bool,
	excludedFamilies, excludedDomains map[string]bool) (candidate, bool) {
	var selected candidate
	var selectedRank [32]byte
	found := false
	for _, value := range view.Candidates[:view.CandidateCount] {
		candidate := candidate{nodeID: value.NodeID, publicKey: value.PublicKey, family: value.Family,
			endpoint: value.Endpoint, capacity: value.Capacity, domain: value.Domain,
			validFrom: value.ValidFrom, validUntil: value.ValidUntil}
		if candidate.domain != role || candidate.capacity == 0 || candidate.nodeID == [32]byte{} ||
			candidate.publicKey == [32]byte{} || candidate.family == "" || excludedIDs[candidate.nodeID] ||
			excludedFamilies[candidate.family] || excludedDomains[candidate.domain] || input.At.Before(candidate.validFrom) ||
			!input.At.Before(candidate.validUntil) || !literalEndpoint(candidate.endpoint) {
			continue
		}
		rank := candidateRank(input.Seed, role, candidate.nodeID)
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

// Validate rejects incomplete, reordered, repeated, or non-literal plans.
func Validate(plan Plan) error {
	if plan.NetworkID == [32]byte{} || plan.Generation == "" || plan.Epoch == 0 || plan.Digest == [32]byte{} ||
		plan.Profile != routeProfile || plan.ViewRoot == [32]byte{} || plan.Seed == [32]byte{} || plan.SelectionAt <= 0 ||
		len(plan.Positions) != len(routeRoles) {
		return errors.New("route must contain every fixed position")
	}
	identities, keys, families, endpoints := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}, map[string]bool{}
	for index, position := range plan.Positions {
		if position.Role != routeRoles[index] || position.Domain != routeRoles[index] || position.Capacity == 0 ||
			position.NodeID == [32]byte{} || position.PublicKey == [32]byte{} || position.Family == "" ||
			!literalEndpoint(position.Endpoint) || identities[position.NodeID] || keys[position.PublicKey] ||
			families[position.Family] || endpoints[position.Endpoint] {
			return errors.New("route position identity, family, domain, or endpoint is invalid")
		}
		identities[position.NodeID], keys[position.PublicKey] = true, true
		families[position.Family], endpoints[position.Endpoint] = true, true
	}
	return nil
}
