package main

import qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"

type manifestCandidate struct {
	NodeID     string `json:"node_id"`
	PublicKey  string `json:"public_key"`
	Family     string `json:"family"`
	Endpoint   string `json:"endpoint"`
	Domain     string `json:"domain"`
	Capacity   uint16 `json:"capacity"`
	ValidFrom  int64  `json:"valid_from"`
	ValidUntil int64  `json:"valid_until"`
}

func resolveCandidates(input *qualification.Case, value manifest) error {
	input.ExcludedIdentities = make([][32]byte, len(value.ExcludedIdentities))
	for index := range value.ExcludedIdentities {
		if err := fixedHex(value.ExcludedIdentities[index], input.ExcludedIdentities[index][:]); err != nil {
			return err
		}
	}
	input.Candidates = make([]qualification.Candidate, len(value.Candidates))
	for index, candidate := range value.Candidates {
		resolved := qualification.Candidate{Family: candidate.Family, Endpoint: candidate.Endpoint, Domain: candidate.Domain,
			Capacity: candidate.Capacity, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil}
		if err := fixedHex(candidate.NodeID, resolved.NodeID[:]); err != nil {
			return err
		}
		if err := fixedHex(candidate.PublicKey, resolved.PublicKey[:]); err != nil {
			return err
		}
		input.Candidates[index] = resolved
	}
	return nil
}
