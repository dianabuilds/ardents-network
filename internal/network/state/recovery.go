package state

import (
	"errors"
	"fmt"

	statestore "github.com/dianabuilds/ardents-network/internal/network/store"
)

func loadGenerationChain(config config, generations map[string]statestore.Generation, name string, seen map[string]bool) (candidateDecision, map[string]bool, error) {
	value, exists := generations[name]
	if !exists || seen[name] || len(seen) >= maximumEpochChain {
		return candidateDecision{}, seen, errors.New("generation chain identity, cycle, or length is invalid")
	}
	seen[name] = true
	parsed, err := parseEpoch(value.Epoch)
	if err != nil {
		return candidateDecision{}, seen, fmt.Errorf("parse generation chain Epoch: %w", err)
	}
	var previous *Snapshot
	if parsed.number > 1 {
		previousName := fmt.Sprintf("%x", parsed.previous)
		prior, updated, priorErr := loadGenerationChain(config, generations, previousName, seen)
		seen = updated
		if priorErr != nil {
			return candidateDecision{}, seen, priorErr
		}
		previous = &prior.snapshot
	}
	decision, err := loadGeneration(config, value, previous)
	if err != nil {
		return candidateDecision{}, seen, err
	}
	if decision.snapshot.Generation != name {
		return candidateDecision{}, seen, errors.New("generation identity does not match its verified digest")
	}
	return decision, seen, nil
}

func recoverMissingCurrent(config config, storage *statestore.Root, generations map[string]statestore.Generation) (*Snapshot, *candidateDecision, error) {
	if len(generations) == 0 {
		return nil, nil, nil
	}
	referenced := make(map[string]bool, len(generations))
	for _, value := range generations {
		parsed, err := parseEpoch(value.Epoch)
		if err != nil {
			return nil, nil, fmt.Errorf("parse orphan generation Epoch: %w", err)
		}
		if parsed.number > 1 {
			referenced[fmt.Sprintf("%x", parsed.previous)] = true
		}
	}
	var tip string
	for name := range generations {
		if referenced[name] {
			continue
		}
		if tip != "" {
			return nil, nil, errors.New("missing current pointer has multiple generation tips")
		}
		tip = name
	}
	if tip == "" {
		return nil, nil, errors.New("missing current pointer has no acyclic generation tip")
	}
	decision, seen, err := loadGenerationChain(config, generations, tip, make(map[string]bool))
	if err != nil {
		return nil, nil, err
	}
	if len(seen) != len(generations) {
		return nil, nil, errors.New("missing current pointer has an orphan generation branch")
	}
	if err := persistDecision(storage, decision, true); err != nil {
		return nil, nil, fmt.Errorf("recover current pointer: %w", err)
	}
	snapshot := decision.snapshot
	return &snapshot, &decision, nil
}
